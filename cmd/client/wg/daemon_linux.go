//go:build linux

package wg

import (
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
)

type WireguardDaemon struct {
	Device         *device.Device
	Uapi           net.Listener
	DefaultGateway netip.Addr
	ServerIP       netip.Prefix
	InterfaceName  string

	// internal variables used for managing the daemon
}

// NewWireguardDaemon returns a new WireguardDaemon NOTE: this is uninitialized
func NewWireguardDaemon() *WireguardDaemon {
	return &WireguardDaemon{}
}

// StartDevice starts the wireguard networking interface used by rVPN
func (d *WireguardDaemon) StartDevice(errs chan error) {
	// use wireguard to create a new device
	interfaceName := "rvpn0"
	deviceMTU := 1420

	// open TUN device
	tun, err := tun.CreateTUN(interfaceName, deviceMTU)
	if err != nil {
		log.Fatalf("failed to create TUN device: %v", err)
	}

	// begin initialization of wireguard on top of TUN device
	logger := device.NewLogger(
		device.LogLevelVerbose,
		fmt.Sprintf("(%s) ", interfaceName),
	)

	// create wireguard device from TUN device
	device := device.NewDevice(tun, conn.NewDefaultBind(), logger)
	logger.Verbosef("created wireguard network interface")

	// start wireguard interface now that routes and interface are set
	log.Println("starting wireguard network interface")

	err = device.Up()
	if err != nil {
		log.Fatalf("failed to bring up device: %v", err)
		os.Exit(2)
	}

	log.Println("wireguard network interface started")

	tunSock, err := ipc.UAPIOpen(interfaceName)
	if err != nil {
		logger.Errorf("failed to open uapi socket: %w", err)
		os.Exit(2)
	}

	uapi, err := ipc.UAPIListen(interfaceName, tunSock)
	if err != nil {
		logger.Errorf("failed to listen on uapi socket: %w", err)
		os.Exit(2)
	}

	// goroutine to listen and accept userspace api connections
	go func() {
		for {
			conn, err := uapi.Accept()
			if err != nil {
				errs <- err
				return
			}
			go device.IpcHandle(conn)
		}
	}()

	logger.Verbosef("UAPI listener started")

	d.Device = device
	d.Uapi = uapi
	d.InterfaceName = interfaceName
}

// UpdateConf updates the configuration of a WireguardDaemon with the provided config
func (d *WireguardDaemon) UpdateConf(wgConf WgConfig) {
	log.Println("starting wireguard network interface configuration")

	err := d.Device.Up()
	if err != nil {
		log.Fatalf("failed to bring up device: %v", err)
	}

	// set ip addresses on the wireguard network interface
	interfaceAddressPrefix := wgConf.ClientIp + wgConf.ClientCidr
	assignInterfaceAddr(d.InterfaceName, interfaceAddressPrefix)

	
}
