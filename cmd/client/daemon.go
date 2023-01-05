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
	"time"

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
	activeRVPNDaemon *RVPNDaemon // rVPN daemon for jrpcHandler to control
	deviceToken      string      // deviceToken for jrpcHandler to AuthN
}

// heartbeatGenerator keeps sending heartbeats until context is cancelled every interval
func heartbeatGenerator(ctx context.Context, interval time.Duration, conn *jsonrpc2.Conn) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// ticker only ticks after interval, manually send a heartbeat first
	var deviceHeartbeatResponse common.DeviceHeartbeatResponse
	err := conn.Call(ctx, common.DeviceHeartbeatMethod, common.DeviceHeartbeatRequest{}, &deviceHeartbeatResponse)
	if err != nil {
		log.Printf("failed to send device heartbeat: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			// it has reached the interval so send heartbeat
			err = conn.Call(ctx, common.DeviceHeartbeatMethod, common.DeviceHeartbeatRequest{}, &deviceHeartbeatResponse)
			if err != nil {
				log.Printf("failed to send device heartbeat: %v", err)
			}

			log.Printf("send device heartbeat to control plane\n")
		case <-ctx.Done():
			// context has been cancelled
			return
		}
	}
}

func (h jrpcHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	switch req.Method {
	case common.GetDeviceAuthMethod:
		// return device AuthN information to rVPN control plane

		conn.Reply(ctx, req.ID, common.GetDeviceAuthResponse{
			Success:     true,
			DeviceToken: h.deviceToken,
		})
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
			Success:   true,
			PublicKey: rVPNState.PublicKey,
		}
		conn.Reply(ctx, req.ID, clientInformationResponse)
	case common.GetServeInformationMethod:
		// return serve information to rVPN control plane

		// get public key from rVPN state
		rVPNState, err := GetRVpnState()
		if err != nil {
			log.Printf("failed to get rVPN state: %v", err)
			conn.Reply(ctx, req.ID, common.GetServeInformationResponse{
				Success: false,
			})
		}

		if rVPNState.PublicKey == "" {
			// public key is not set, regenerate wg keys
			privateKey, publicKey, err := wg.GenerateKeyPair()
			if err != nil {
				log.Printf("failed to generate new wireguard keypair: %v", err)
				conn.Reply(ctx, req.ID, common.GetServeInformationResponse{
					Success: false,
				})
			}

			rVPNState.PrivateKey = privateKey
			rVPNState.PublicKey = publicKey

			err = SetRVpnState(rVPNState)
			if err != nil {
				log.Printf("failed to save rVPN state: %v", err)
				conn.Reply(ctx, req.ID, common.GetServeInformationResponse{
					Success: false,
				})
			}
		}

		// we must have a saved public key in rVPN state, return information to control plane
		clientInformationResponse := common.GetServeInformationResponse{
			Success:       true,
			PublicKey:     rVPNState.PublicKey,
			PublicVpnPort: "21820", // TODO: allow this to be overriden with config flags
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
		userConfig := wg.ClientWgConfig{
			ClientPrivateKey: rVPNState.PrivateKey,
			ServerPublicKey:  connectServerRequest.ServerPublicKey,
			ClientIp:         connectServerRequest.ClientIp,
			ClientCidr:       connectServerRequest.ClientCidr,
			ServerIp:         connectServerRequest.ServerIp,
			ServerPort:       connectServerRequest.ServerPort,
			DnsIp:            connectServerRequest.DnsIp,
		}
		h.activeRVPNDaemon.wireguardDaemon.UpdateClientConf(userConfig)

		// TODO: wait then run a check to ensure connection is healthy, otherwise abort

		h.activeRVPNDaemon.status = StatusConnected

		log.Printf("daemon successfully connected to rVPN target server")
		conn.Reply(ctx, req.ID, common.ConnectServerResponse{
			Success: true,
		})

		// launch goroutine to send heartbeat to keep WS alive
		// NOTE: context is of the jrpc connection which should be kept alive
		go heartbeatGenerator(ctx, 30*time.Second, conn)
	case common.ServeVPNMethod:
		// NOTE: the serve vpn code path should only be triggered on Linux devices
		serveVPNHandler(ctx, h, conn, req)
	case common.AppendVPNPeersMethod:
		// NOTE: the append peer code path should only be triggered on Linux devices
		appendVPNPeersHandler(ctx, h, conn, req)
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

	// now we are authenticated, create jrpc connection on top of websocket stream
	jrpcConn := jsonrpc2.NewConn(ctx, jrpc.NewObjectStream(conn), jrpcHandler{
		activeRVPNDaemon: r,
		deviceToken:      args.DeviceToken,
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
