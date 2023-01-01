package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"net/rpc"
	"os"
	"os/signal"
	"syscall"

	"github.com/redpwn/rvpn/cmd/client/jrpc"
	"github.com/redpwn/rvpn/cmd/client/wg"
	"github.com/redpwn/rvpn/common"
	"github.com/sourcegraph/jsonrpc2"
	"nhooyr.io/websocket"
)

// this is the windows daemon which runs the rVPN daemon as well as rVPN wireguard daemon
// the rVPN daemon manages the long lived connection to the control plane and executing commands

type RVPNStatus int

const (
	StatusConnected RVPNStatus = iota
	StatusDisconnected
	StatusServing
)

type ConnectRequest struct {
	Profile     string
	DeviceToken string
}

type ServeRequest struct {
	Profile     string
	DeviceToken string
}

// RVPNDaemon represents a rVPN daemon instance
type RVPNDaemon struct {
	status               RVPNStatus
	activeControlPlaneWs *websocket.Conn
	activeProfile        string
	jrpcConn             *jsonrpc2.Conn

	// internal variables used for underlying control
	wireguardDaemon *wg.WireguardDaemon
	manualTerm      chan int
}

func NewRVPNDaemon() *RVPNDaemon {
	return &RVPNDaemon{
		status:     StatusDisconnected,
		manualTerm: make(chan int),
	}
}

// jsonRPC handler for daemon
type jrpcHandler struct {
	activeRVPNDaemon *RVPNDaemon
}

func (h jrpcHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	switch req.Method {
	case common.GetClientInformationMethod:
		// return client information to rVPN control plane

		// get public key from rVPN state
		rVPNState, err := GetRVpnState()
		if err != nil {
			log.Printf("failed to get rVPN state: %v", err)
			conn.Reply(ctx, req.ID, common.GetClientInformationResponse{
				Success: false,
			})
		}

		if rVPNState.PublicKey == "" {
			// public key is not set, regenerate wg keys
			privateKey, publicKey, err := wg.GenerateKeyPair()
			if err != nil {
				log.Printf("failed to generate new wireguard keypair: %v", err)
				conn.Reply(ctx, req.ID, common.GetClientInformationResponse{
					Success: false,
				})
			}

			rVPNState.PrivateKey = privateKey
			rVPNState.PublicKey = publicKey

			err = SetRVpnState(rVPNState)
			if err != nil {
				log.Printf("failed to save rVPN state: %v", err)
				conn.Reply(ctx, req.ID, common.GetClientInformationResponse{
					Success: false,
				})
			}
		}

		// we must have a saved public key in rVPN state, return information to control plane
		clientInformationResponse := common.GetClientInformationResponse{
			PublicKey: rVPNState.PublicKey,
		}
		conn.Reply(ctx, req.ID, clientInformationResponse)
	case common.ConnectServerMethod:
		// connect to server with information provided from rVPN control plane

		// get pubkey and privkey from rVPN state
		rVPNState, err := GetRVpnState()
		if err != nil {
			log.Printf("failed to get rVPN state: %v", err)
			conn.Reply(ctx, req.ID, common.ConnectServerResponse{
				Success: false,
			})
		}

		if rVPNState.PublicKey == "" || rVPNState.PrivateKey == "" {
			// pubkey or privkey is not set, error
			log.Printf("pubkey or privkey is not set, check rVPN state")
			conn.Reply(ctx, req.ID, common.ConnectServerResponse{
				Success: false,
			})
		}

		// parse information from jrpc request
		var connectServerRequest common.ConnectServerRequest
		err = json.Unmarshal(*req.Params, &connectServerRequest)
		if err != nil {
			log.Printf("failed to unmarshal connectserver request params: %v", err)
			conn.Reply(ctx, req.ID, common.ConnectServerResponse{
				Success: false,
			})
		}

		// validate pubkey from connectServerRequest matches local pubkey
		if connectServerRequest.ClientPublicKey != rVPNState.PublicKey {
			log.Printf("pubkey has fallen out of sync between control plane and device, try again")
			conn.Reply(ctx, req.ID, common.ConnectServerResponse{
				Success: false,
			})
		}

		// update rVPN wireguard config with instructions from rVPN control plane
		userConfig := wg.WgConfig{
			PrivateKey: rVPNState.PrivateKey,
			PublicKey:  connectServerRequest.ServerPublicKey,
			ClientIp:   connectServerRequest.ClientIp,
			ClientCidr: connectServerRequest.ClientCidr,
			ServerIp:   connectServerRequest.ServerIp,
			ServerPort: connectServerRequest.ServerPort,
			DnsIp:      connectServerRequest.DnsIp,
		}
		h.activeRVPNDaemon.wireguardDaemon.UpdateConf(userConfig)

		// TODO: wait then run a check to ensure connection is healthy, otherwise abort

		h.activeRVPNDaemon.status = StatusConnected

		// TODO: launch goroutine to send heartbeat to keep WS alive
	default:
		log.Printf("unknown jrpc request method: %s\n", req.Method)
	}
}

/* rVPN daemon rpc handlers */

// Status returns the current status of the rVPN daemon
func (r *RVPNDaemon) Status(args string, reply *RVPNStatus) error {
	*reply = r.status

	return nil
}

// Connect is responsible for creating WebSocket connection to control-plane
func (r *RVPNDaemon) Connect(args ConnectRequest, reply *bool) error {
	// create long-lived WebSocket connection acting as jrpc channel between client and control plane
	ctx := context.Background()

	websocketURL := RVPN_CONTROL_PLANE_WS + "/api/v1/target/" + args.Profile + "/connect"
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

func (r *RVPNDaemon) Disconnect(args string, reply *bool) error {
	r.wireguardDaemon.Disconnect()
	r.jrpcConn.Close()
	r.status = StatusDisconnected

	*reply = true
	return nil
}

func (r *RVPNDaemon) Start() {
	log.Println("starting rVPN wireguard daemon...")

	wireguardDaemon := wg.NewWireguardDaemon()
	r.wireguardDaemon = wireguardDaemon

	errs := make(chan error)
	term := make(chan os.Signal, 1)

	// start the wireguard interface device
	wireguardDaemon.StartDevice(errs)

	// start RPC server; TODO: investigate if this is a good pattern
	rpc.Register(r)

	tcpAddr, err := net.ResolveTCPAddr("tcp", ":52370")
	if err != nil {
		log.Fatalf("failed to resolve tcp address: %v", err)
	}

	listener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		log.Fatalf("failed to start tcp listener: %v", err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				continue
			}
			rpc.ServeConn(conn)
		}
	}()

	log.Println("started rVPN daemon RPC server")

	log.Println("rVPN wireguard daemon is running")

	// wait for program to terminate via signal, interrupt, or error
	signal.Notify(term, syscall.SIGTERM)
	signal.Notify(term, os.Interrupt)

	select {
	case <-term:
	case <-errs:
	case <-r.manualTerm:
	case <-wireguardDaemon.Device.Wait():
	}

	// once we receive termination signal, shutdown wg device
	wireguardDaemon.ShutdownDevice()
	log.Println("rVPN wireguard daemon stopped")
}

// Stop stops the rVPN daemon process
func (r *RVPNDaemon) Stop() {
	log.Println("stopping rVPN wireguard daemon...")
	r.manualTerm <- 1
}
