package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	"github.com/redpwn/rvpn/cmd/control-plane/jrpc"
	"github.com/redpwn/rvpn/common"
	"github.com/sourcegraph/jsonrpc2"
	"go.uber.org/zap"
)

// jsonRPC handler for client devices
type jrpcClientHandler struct {
	heartbeatChan chan int

	// internal constructs
	log *zap.Logger
}

func (h jrpcClientHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	switch req.Method {
	case common.DeviceHeartbeatMethod:
		// acknowledge heartbeat and send message to channel
		h.heartbeatChan <- 1
		conn.Reply(ctx, req.ID, common.DeviceHeartbeatResponse{
			Success: true,
		})
	default:
		h.log.Info("received unknown jrpc command")
	}
}

// WebSocket entry point for JSON RPC between control plane and rVPN client devices
func (a *app) clientConnect(c *fiber.Ctx) error {
	target := c.Params("target")
	if target == "" {
		return c.Status(400).JSON(ErrorResponse("target must not be empty"))
	}

	handler := websocket.New(func(wc *websocket.Conn) {
		// TODO: verify that this is the correct way to maintain context in a websocket
		ctx, cancelFunc := context.WithCancel(context.Background())
		defer func() {
			cancelFunc()
			wc.Close()
		}()

		// create jrpc connection on top of websocket stream; each connection has its own handler instance
		heartbeatChan := make(chan int, 2) // buffer 2 heartbeats
		jrpcConn := jsonrpc2.NewConn(c.Context(), jrpc.NewObjectStream(wc), jrpcClientHandler{
			heartbeatChan: heartbeatChan,
			log:           a.log,
		})

		// request device auth
		var getDeviceAuthResponse common.GetDeviceAuthResponse
		err := jrpcConn.Call(ctx, common.GetDeviceAuthMethod, common.GetDeviceAuthRequest{}, &getDeviceAuthResponse)
		if err != nil {
			a.log.Error("failed to call getdeviceauth via jrpc", zap.Error(err))
			return
		}

		deviceId, err := a.ValidateDeviceToken(getDeviceAuthResponse.DeviceToken)
		if err != nil {
			a.log.Error("failed to validate device token")
			return
		}

		// TODO(authentication): proper authZ (ensure deviceId is allowed to access target server)
		if deviceId == "" {
			a.log.Info("something went wrong; this is a stub for authZ")
		}

		// get target information and ensure target is alive
		rVPNTarget, err := a.db.getTargetByName(ctx, target)
		if err != nil {
			a.log.Error("failed to get target by name for client connection", zap.Error(err))
			return
		}

		if !targetServerAlive(rVPNTarget, a.connMan) {
			// server is not alive, we cannot make a connection
			// TODO: better way to inform client is to have this be an error for connect command (architectural issue)
			a.log.Info("target server is not alive, we cannot connect")
			return
		}

		// we are now authentciated and target server is alive

		// request client information (pubkey) via jrpc
		var clientInformationResponse common.GetClientInformationResponse
		err = jrpcConn.Call(ctx, common.GetClientInformationMethod, common.GetClientInformationRequest{}, &clientInformationResponse)
		if err != nil {
			a.log.Error("failed to call getclientinformation via jrpc", zap.Error(err))
			return
		}

		// we must ensure there is a connection for the device
		appendPeerToVPNServer := false
		deviceConnection, err := a.db.getConnection(ctx, target, deviceId)
		if err != nil {
			a.log.Error("failed to check if connection exists", zap.Error(err))
		}

		if deviceConnection.id != "" {
			// device connection already exists, verify that pubkey is what we expect for device
			if deviceConnection.pubkey != clientInformationResponse.PublicKey {
				// rVPN control plane pubkey is out of sync with client, re-sync control plane (clientResponseOverrides)
				a.log.Info("device connection pubkey out of sync")

				err = syncConnectionPubkey(ctx, a.db, deviceConnection, clientInformationResponse.PublicKey)
				if err != nil {
					a.log.Error("failed to sync connection and device pubkey", zap.Error(err))
					return
				}

				deviceConnection.pubkey = clientInformationResponse.PublicKey

				// re-sync client to target VPN server by appending new peer
				appendPeerToVPNServer = true

				// TODO: instruct rVPN server to remove old peer
			}
		} else {
			// device connection does not exist yet, create connection using information from jrpc

			// get next free ip for the target
			clientIp, clientCidr, err := getNextClientIp(ctx, a.db, target)
			if err != nil {
				a.log.Error("failed to get next client ip", zap.Error(err))
				return
			}

			// create connection in database
			newUUID := uuid.New().String()
			deviceConnection = RVPNConnection{
				id:         newUUID,
				target:     target,
				deviceId:   deviceId,
				pubkey:     clientInformationResponse.PublicKey,
				clientIp:   clientIp,
				clientCidr: clientCidr,
			}
			err = createConnection(ctx, a.db, deviceConnection)
			if err != nil {
				a.log.Error("failed to create connection", zap.Error(err))
				return
			}

			// append new client to target VPN server so VPN server knows about client
			appendPeerToVPNServer = true
		}

		if appendPeerToVPNServer {
			// if needed, instruct vpn server to add client as a peer
			a.log.Info("appending client to VPN server as a peer")
			vpnServerConn := a.connMan.getVPNServerConn(target)
			if vpnServerConn == nil {
				a.log.Error("vpn server connection is not alive, cannot add new peer")
			}

			appendVPNPeersRequest := common.AppendVPNPeersRequest{
				Peers: []common.WireGuardPeer{{
					PublicKey:   deviceConnection.pubkey,
					AllowedIP:   deviceConnection.clientIp,
					AllowedCidr: deviceConnection.clientCidr,
				}},
			}

			var appendVPNPeersResponse common.AppendVPNPeersResponse
			err = vpnServerConn.Call(ctx, common.AppendVPNPeersMethod, appendVPNPeersRequest, &appendVPNPeersResponse)
			if err != nil {
				a.log.Error("failed to call appendvpnpeers via jrpc for new device connect", zap.Error(err))
			}
		}

		// device connection is complete, jrpc client to connect to rVPN server
		intServerVpnPort, err := strconv.Atoi(rVPNTarget.serverPublicVpnPort)
		if err != nil {
			a.log.Error("failed to convert vpn port to int", zap.Error(err))
			return
		}

		connectServerRequest := common.ConnectServerRequest{
			ServerPublicKey: rVPNTarget.serverPubkey,
			ClientPublicKey: deviceConnection.pubkey,
			ClientIp:        deviceConnection.clientIp,
			ClientCidr:      deviceConnection.clientCidr,
			ServerIp:        rVPNTarget.serverPublicIp,
			ServerPort:      intServerVpnPort,
			DnsIp:           rVPNTarget.dnsIp,
		}

		var connectServerResponse common.ConnectServerResponse
		err = jrpcConn.Call(ctx, common.ConnectServerMethod, connectServerRequest, &connectServerResponse)
		if err != nil {
			a.log.Error("failed to call connectserver via jrpc", zap.Error(err))
		}

		// TODO: remove debug
		fmt.Println("issued connect server with following info", connectServerRequest.ServerIp, connectServerRequest.ServerPublicKey, connectServerRequest.ClientPublicKey)

		// save the jrpc connection for the rvpn client to the connection manager
		a.connMan.setVPNClientConn(target, jrpcConn)

		// block to keep WebSocket alive (stale timeout of 3 minutes)
		blockUntilStale(ctx, heartbeatChan, 3*time.Minute)
	})

	return handler(c)
}

// createConnection creates a rVPN control plane connection
func createConnection(ctx context.Context, db *RVPNDatabase, rVPNConnection RVPNConnection) error {
	existingRVPNConnection, err := db.getConnection(ctx, rVPNConnection.target, rVPNConnection.deviceId)
	if err != nil {
		return err
	}

	// if connection has already been created then we can early exit
	if existingRVPNConnection.id != "" {
		return nil
	}

	// create new connection and save it to the database
	_, err = db.createConnection(ctx, rVPNConnection.id, rVPNConnection.target, rVPNConnection.deviceId, rVPNConnection.pubkey, rVPNConnection.clientIp, rVPNConnection.clientCidr)
	if err != nil {
		return err
	}

	return nil
}
