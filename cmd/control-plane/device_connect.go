package main

import (
	"context"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	"github.com/redpwn/rvpn/cmd/control-plane/jrpc"
	"github.com/redpwn/rvpn/common"
	"github.com/sourcegraph/jsonrpc2"
	"go.uber.org/zap"
)

/*
type rVpnServer struct {
	conn *jsonrpc2.Conn
}

var clients = make(map[string]rVpnServer)
*/

// WebSocket entry point for JSON RPC between control plane and rVPN client devices
func (a *app) clientConnection(c *fiber.Ctx) error {
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

		// we first wait for the server to send their deviceToken and authenticate
		messageType, message, err := wc.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				a.log.Error("failed to read websocket message", zap.Error(err))
			}

			return
		}

		var deviceId string
		if messageType == websocket.TextMessage {
			// message is the device token, validate and extract device
			deviceId, err = a.ValidateDeviceToken(string(message))
			if err != nil {
				a.log.Error("failed to validate device token")
				return
			}
		} else {
			a.log.Error("received non text message after initializing connection")
			return
		}

		// get target information and sure target is alive
		rVPNTarget, err := a.db.getTargetByName(ctx, target)
		if err != nil {
			a.log.Error("failed to get target by name for client connection", zap.Error(err))
			return
		}

		if !targetServerAlive(rVPNTarget) {
			// server is not alive, we cannot make a connection
			// TODO: better way to inform client is to have this be an error for connect command (architectural issue)
			a.log.Info("target server is not alive, we cannot connect")
			return
		}

		// we are now authentciated, create jrpc connection on top of websocket stream
		jrpcConn := jsonrpc2.NewConn(c.Context(), jrpc.NewObjectStream(wc), nil)

		// request client information (pubkey) via jrpc
		var clientInformationResponse common.GetClientInformationResponse
		err = jrpcConn.Call(ctx, common.GetClientInformationMethod, common.GetClientInformationRequest{}, &clientInformationResponse)
		if err != nil {
			a.log.Error("failed to call getclientinformation via jrpc", zap.Error(err))
			return
		}

		// we must ensure there is a connection for the device
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
		}

		// the device connection exists, jrpc client to connect to rVPN server
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
	})

	return handler(c)
}

// createConnection creates a rVPN control plane connection
func createConnection(ctx context.Context, db *RVPNDatabase, rVPNConnection RVPNConnection) error {
	rVPNConnection, err := db.getConnection(ctx, rVPNConnection.target, rVPNConnection.deviceId)
	if err != nil {
		return err
	}

	// if connection has already been created then we can early exit
	if rVPNConnection.id != "" {
		return nil
	}

	// create new connection and save it to the database
	_, err = db.createConnection(ctx, rVPNConnection.id, rVPNConnection.target, rVPNConnection.deviceId, rVPNConnection.pubkey, rVPNConnection.clientIp, rVPNConnection.clientCidr)
	if err != nil {
		return err
	}

	return nil
}
