package main

import (
	"context"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/redpwn/rvpn/cmd/control-plane/jrpc"
	"github.com/redpwn/rvpn/common"
	"github.com/sourcegraph/jsonrpc2"
	"go.uber.org/zap"
)

// jsonRPC handler for serving devices
type jrpcServeHandler struct {
	heartbeatChan chan int

	// internal constructs
	log *zap.Logger
}

func (h jrpcServeHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
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

// WebSocket entry point for JSON RPC between control plane and rVPN serving devices
func (a *app) clientServe(c *fiber.Ctx) error {
	target := c.Params("target")
	if target == "" {
		return c.Status(400).JSON(ErrorResponse("target must not be empty"))
	}

	clientPublicIP := c.IPs()[0]

	handler := websocket.New(func(wc *websocket.Conn) {
		// TODO: verify that this is the correct way to maintain context in a websocket
		ctx, cancelFunc := context.WithCancel(context.Background())
		defer func() {
			cancelFunc()
			wc.Close()

			// TODO: clean up self from connMan
		}()

		// we are now authentciated, create jrpc connection on top of websocket stream
		heartbeatChan := make(chan int)
		jrpcConn := jsonrpc2.NewConn(c.Context(), jrpc.NewObjectStream(wc), jrpcServeHandler{
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

		// get target information and ensure target exists
		rVPNTarget, err := a.db.getTargetByName(ctx, target)
		if err != nil {
			a.log.Error("failed to get target by name for client connection", zap.Error(err))
			return
		}

		if rVPNTarget == nil {
			a.log.Error("failed to get target as it does not exist", zap.String("target", target))
		}

		// NOTE: current behavior is to override if connection is already being served
		// TODO: investigate if there should be a targetServeAlive check + force flag

		// we are now authenticated and have confirmed that the vpn target exists

		// request client information (pubkey, public vpn port) via jrpc
		var serveInformationResponse common.GetServeInformationResponse
		err = jrpcConn.Call(ctx, common.GetServeInformationMethod, common.GetServeInformationRequest{}, &serveInformationResponse)
		if err != nil {
			a.log.Error("failed to call getserveinformation via jrpc", zap.Error(err))
			return
		}

		a.log.Info("serve client ip: " + clientPublicIP)

		// TODO: data architecture decision, does VPN server count as a connection? this *could*
		// simplify distribution of IPs but may introduce other problems

		// update backend target
		rVPNTarget.serverPubkey = serveInformationResponse.PublicKey
		rVPNTarget.serverPublicIp = clientPublicIP
		rVPNTarget.serverPublicVpnPort = serveInformationResponse.PublicVpnPort

		a.db.updateTarget(ctx, target, rVPNTarget)

		// backend data is updated, instruct jrpc client to serve as VPN server
		intServerVpnPort, err := strconv.Atoi(rVPNTarget.serverPublicVpnPort)
		if err != nil {
			a.log.Error("failed to convert vpn port to int", zap.Error(err))
			return
		}

		// add all current connections to target as peers
		rVPNPeers := []common.WireGuardPeer{}

		targetConnections, err := a.db.getConnectionsByTarget(ctx, target)
		if err != nil {
			a.log.Error("failed to get target connections", zap.Error(err))
		}

		for _, targetConnection := range targetConnections {
			wireguardPeer := common.WireGuardPeer{
				PublicKey:   targetConnection.pubkey,
				AllowedIP:   targetConnection.clientIp,
				AllowedCidr: targetConnection.clientCidr,
			}

			rVPNPeers = append(rVPNPeers, wireguardPeer)
		}

		serveVPNRequest := common.ServeVPNRequest{
			ServerPublicKey:     rVPNTarget.serverPubkey,
			ServerInternalIp:    rVPNTarget.serverInternalIp,
			ServerInternalCidr:  rVPNTarget.serverInternalCidr,
			ServerPublicVPNPort: intServerVpnPort,
			Peers:               rVPNPeers,
		}

		var serveVPNResponse common.ServeVPNResponse
		err = jrpcConn.Call(ctx, common.ServeVPNMethod, serveVPNRequest, &serveVPNResponse)
		if err != nil {
			a.log.Error("failed to call connectserver via jrpc", zap.Error(err))
		}

		a.log.Info("successfully issued jrpc command to client to serve as VPN server")

		// save the jrpc connection for the rvpn server to the connection manager
		a.connMan.setVPNServerConn(target, jrpcConn)

		// TODO: broadcast to all clients on the profile to connect to the new VPN server

		// block to keep WebSocket alive (stale timeout of 3 minutes)
		blockUntilStale(ctx, heartbeatChan, 3*time.Minute)
	})

	return handler(c)
}
