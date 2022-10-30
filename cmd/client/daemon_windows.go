//go:build windows

package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os"
	"os/signal"
	"syscall"

	"github.com/redpwn/rvpn/cmd/client/wg"
)

// this is the windows daemon which runs the rVPN daemon as well as rVPN wireguard daemon
// the daemon manages the long lived connection to the control plane and executing commands

type RVPNStatus int

const (
	StatusConnected RVPNStatus = iota
	StatusDisconnected
)

// RVPNDaemon represents a rVPN daemon instance
type RVPNDaemon struct {
	status RVPNStatus

	// channels used for control
	manualTerm chan int
}

func NewRVPNDaemon() *RVPNDaemon {
	return &RVPNDaemon{
		status:     StatusDisconnected,
		manualTerm: make(chan int),
	}
}

// EnsureDaemonStarted checks if the daemon is started
func EnsureDaemonStarted() error {
	// elevate and run the command "rvpn daemon"
	// TODO: complete this function
	return nil
}

func (r *RVPNDaemon) Status(args string, reply *RVPNStatus) error {
	fmt.Println("requested status", r.status)
	r.status = StatusDisconnected

	*reply = r.status

	return nil
}

func (r *RVPNDaemon) Connect() error {
	return nil
}

func (r *RVPNDaemon) Start() {
	log.Println("starting windows rVPN wireguard daemon...")

	wireguardDaemon := wg.NewWireguardDaemon()

	errs := make(chan error)
	term := make(chan os.Signal, 1)

	// start the wireguard interface device
	wireguardDaemon.StartDevice(errs)

	// TODO: accept WgConfig via rpc to configure device via wgctrl
	userConfig := wg.WgConfig{
		PrivateKey: "--",
		PublicKey:  "Xb5+rEyb4eozBWYruk5iA7shr8miaQMka937dagG20c=",
		ClientIp:   "10.8.0.2",
		ClientCidr: "/24",
		ServerIp:   "144.172.71.160",
		ServerPort: 21820,
		DnsIp:      "1.1.1.1",
	}
	wireguardDaemon.UpdateConf(userConfig)

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

	log.Println("windows rVPN wireguard daemon is running")

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
	log.Println("windows rVPN wireguard daemon stopped")
}

// Stop stops the rVPN daemon process
func (r *RVPNDaemon) Stop() {
	log.Println("stopping windows rVPN wireguard daemon...")
	r.manualTerm <- 1
}
