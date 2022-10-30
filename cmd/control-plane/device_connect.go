package main

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
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
func (a *app) serveConnection(c *fiber.Ctx) error {
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

		// we are now authentciated, create jrpc connection on top of websocket stream
		jrpcConn := jsonrpc2.NewConn(c.Context(), jrpc.NewObjectStream(wc), nil)

		// we must ensure there is a connection for the device
		deviceConnectionExists, err := connectionExists(ctx, a.db, target, deviceId)
		if err != nil {
			a.log.Error("failed to check if connection exists", zap.Error(err))
		}

		if !deviceConnectionExists {
			// device connection does not exist yet, we must jrpc client and ask for information
			var clientInformationResponse common.GetClientInformationResponse
			jrpcConn.Call(ctx, "get_client_information", common.GetClientInformationRequest{}, &clientInformationResponse)
		}

		// at this point the device connection exists, jrpc client to connect to rVPN server
		jrpcConn.Call(ctx, "connect_server", nil, nil)
	})

	return handler(c)
}

func connectionExists(ctx context.Context, db *RVPNDatabase, targetName, deviceId string) (bool, error) {
	connectionId, err := db.getConnection(ctx, targetName, deviceId)
	if err != nil {
		return false, err
	}

	// connection has already been created if the connectionId is not empty
	if connectionId != "" {
		return true, nil
	} else {
		return false, nil
	}
}

func createConnection(ctx context.Context, db *RVPNDatabase, targetName, deviceId string) error {
	connectionId, err := db.getConnection(ctx, targetName, deviceId)
	if err != nil {
		return err
	}

	// if connection has already been created then we can early exit
	if connectionId != "" {
		return nil
	}

	return nil
}
