//go:build linux

package main

import (
	"context"
	"encoding/json"
	"log"

	"github.com/redpwn/rvpn/cmd/client/jrpc"
	"github.com/redpwn/rvpn/cmd/client/wg"
	"github.com/redpwn/rvpn/common"
	"github.com/sourcegraph/jsonrpc2"
	"nhooyr.io/websocket"
)

// EnsureDaemonStarted checks if the daemon is started
func EnsureDaemonStarted() error {
	// elevate and run the command "rvpn daemon"
	// TODO: complete this function
	return nil
}

// Serve instructs the rVPN daemon to act as a target VPN server
func (r *RVPNDaemon) Serve(args ServeRequest, reply *bool) error {
	// create long-lived WebSocket connection acting as jrpc channel between client and control plane
	ctx := context.Background()

	websocketURL := RVPN_CONTROL_PLANE_WS + "/api/v1/target/" + args.Profile + "/serve"
	conn, _, err := websocket.Dial(ctx, websocketURL, nil)
	if err != nil {
		log.Printf("failed to connect to rVPN control plane web socket: %v", err)
		*reply = false
		return nil
	}

	r.activeControlPlaneWs = conn
	r.activeProfile = args.Profile

	// send device token to authenticate with control plane
	err = conn.Write(ctx, websocket.MessageText, []byte(args.DeviceToken))
	if err != nil {
		log.Printf("failed to write device token to control plane web socket: %v", err)
		*reply = false
		return nil
	}

	// now we are authenticated, create jrpc connection on top of websocket stream
	jrpcConn := jsonrpc2.NewConn(ctx, jrpc.NewObjectStream(conn), jrpcHandler{
		activeRVPNDaemon: r,
	})

	r.jrpcConn = jrpcConn

	*reply = true
	return nil
}

// serveVPNHandler is responsible for handling when control plane instructs the client to serve a VPN
func serveVPNHandler(ctx context.Context, h jrpcHandler, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	// start acting as VPN server with the given arguments from the control plane
	// NOTE: this code path should only be triggered on Linux devices

	// get pubkey and privkey from rVPN state
	rVPNState, err := GetRVpnState()
	if err != nil {
		log.Printf("failed to get rVPN state: %v", err)
		conn.Reply(ctx, req.ID, common.ServeVPNResponse{
			Success: false,
		})
	}

	if rVPNState.PublicKey == "" || rVPNState.PrivateKey == "" {
		// pubkey or privkey is not set, error
		log.Printf("pubkey or privkey is not set, check rVPN state")
		conn.Reply(ctx, req.ID, common.ServeVPNResponse{
			Success: false,
		})
	}

	// parse information from jrpc request
	var serveVPNRequest common.ServeVPNRequest
	err = json.Unmarshal(*req.Params, &serveVPNRequest)
	if err != nil {
		log.Printf("failed to unmarshal servevpn request params: %v", err)
		conn.Reply(ctx, req.ID, common.ServeVPNResponse{
			Success: false,
		})
	}

	// validate pubkey from serveVPNRequest matches local pubkey
	if serveVPNRequest.ServerPublicKey != rVPNState.PublicKey {
		log.Printf("pubkey has fallen out of sync between control plane and device, try again")
		conn.Reply(ctx, req.ID, common.ServeVPNResponse{
			Success: false,
		})
	}

	wgPeers := []wg.WireGuardPeer{}
	for _, clientPeer := range serveVPNRequest.Peers {
		newPeer := wg.WireGuardPeer{
			PublicKey:   clientPeer.PublicKey,
			AllowedIP:   clientPeer.AllowedIP,
			AllowedCidr: clientPeer.AllowedCidr,
		}

		wgPeers = append(wgPeers, newPeer)
	}

	serveConfig := wg.ServeWgConfig{
		PrivateKey:   rVPNState.PrivateKey,
		ListenPort:   21820, // TODO: allow this to be changed with config flags
		InternalIp:   serveVPNRequest.ServerInternalIp,
		InternalCidr: serveVPNRequest.ServerInternalCidr,
		Peers:        wgPeers,
	}

	h.activeRVPNDaemon.wireguardDaemon.UpdateServeConf(serveConfig)
	h.activeRVPNDaemon.status = StatusServing

	log.Printf("daemon successfully serving as rVPN target VPN server")
	conn.Reply(ctx, req.ID, common.ServeVPNResponse{
		Success: true,
	})

	// TODO: launch goroutine to send heartbeat to keep WS alive
}

// appendVPNPeersHandler is responsible for append peers to the Wireguard config
func appendVPNPeersHandler(ctx context.Context, h jrpcHandler, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	// parse information from jrpc request
	var appendVPNPeersRequest common.AppendVPNPeersRequest
	err := json.Unmarshal(*req.Params, &appendVPNPeersRequest)
	if err != nil {
		log.Printf("failed to unmarshal appendvpnpeers request params: %v", err)
		conn.Reply(ctx, req.ID, common.AppendVPNPeersResponse{
			Success: false,
		})
	}

	wgPeers := []wg.WireGuardPeer{}
	for _, requestPeer := range appendVPNPeersRequest.Peers {
		wgPeer := wg.WireGuardPeer{
			PublicKey:   requestPeer.PublicKey,
			AllowedIP:   requestPeer.AllowedIP,
			AllowedCidr: requestPeer.AllowedCidr,
		}

		wgPeers = append(wgPeers, wgPeer)
	}

	h.activeRVPNDaemon.wireguardDaemon.AppendPeers(wgPeers)
	log.Printf("daemon successfully appended new peers to rVPN target VPN server")
}
