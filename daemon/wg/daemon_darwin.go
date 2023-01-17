//go:build darwin

package wg

import (
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type routeInfo struct {
	ipAddressPrefix string
	interfaceName   string
}

type WireguardDaemon struct {
	Device           *device.Device
	Uapi             net.Listener
	DefaultIFaceName string
	DefaultGatewayIP string
	ServerIP         netip.Prefix
	ControlPlaneIP   netip.Prefix
	InterfaceName    string

	// internal variables used for managing the daemon
	appendedRoutes []routeInfo
}

// NewWireguardDaemon returns a new WireguardDaemon NOTE: this is uninitialized
func NewWireguardDaemon() *WireguardDaemon {
	return &WireguardDaemon{}
}

// StartDevice starts the wireguard networking interface used by rVPN
func (d *WireguardDaemon) StartDevice(errs chan error) error {
	// use wireguard to create a new device
	interfaceName := "utun140"
	deviceMTU := 1420

	// open TUN device
	tun, err := tun.CreateTUN(interfaceName, deviceMTU)
	if err != nil {
		return fmt.Errorf("failed to create TUN device: %w", err)
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
		return fmt.Errorf("failed to bring up device: %w", err)
	}

	log.Println("wireguard network interface started")

	tunSock, err := ipc.UAPIOpen(interfaceName)
	if err != nil {
		return fmt.Errorf("failed to open uapi socket: %w", err)
	}

	uapi, err := ipc.UAPIListen(interfaceName, tunSock)
	if err != nil {
		return fmt.Errorf("failed to listen on uapi socket: %w", err)
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

	return nil
}

// UpdateClientConf updates the configuration of a WireguardDaemon with the provided config for rVPN clients
// NOTE: must route controlPlaneAddr to default gateway to preserve control plane WebSocket
func (d *WireguardDaemon) UpdateClientConf(wgConf ClientWgConfig, controlPlaneAddr string) {
	log.Println("starting wireguard network interface configuration for clients")

	// bring the interface and device up
	cmd := exec.Command("ifconfig", d.InterfaceName, "up")
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("interface up command \"%v\" failed with output %s and error: ", cmd.String(), out)
	}

	err := d.Device.Up()
	if err != nil {
		log.Fatalf("failed to bring up device: %v", err)
	}

	// parse out server and control plane ips
	serverIP, err := netip.ParsePrefix(wgConf.ServerIp + "/32")
	if err != nil {
		log.Fatalf("failed to parse server ip: %v", err)
	}

	d.ServerIP = serverIP

	controlPlaneIP, err := netip.ParsePrefix(controlPlaneAddr + "/32")
	if err != nil {
		log.Fatalf("failed to parse control plane ip: %v", err)
	}

	d.ControlPlaneIP = controlPlaneIP

	// find default adapater and add a highest priority route so traffic to vpn host is not routed through wireguard interface
	currDefaultIFace, currDefaultGateway, err := findDefaultInterface()
	if err != nil {
		log.Fatalf("failed to find default interface: %v", err)
	}

	if d.DefaultIFaceName == "" {
		log.Println("updating default interface...")
		// default interface has not yet been defined
		d.DefaultIFaceName = currDefaultIFace
		d.DefaultGatewayIP = currDefaultGateway

		// set target server route on default interface as it has not been set yet
		err = routeAddGateway(d.ServerIP.String(), currDefaultGateway)
		if err != nil {
			log.Fatalf("failed to add server IP to default interface: %v", err)
		}

		// set control plane route on default interface
		err = routeAddGateway(d.ControlPlaneIP.String(), currDefaultGateway)
		if err != nil {
			log.Fatalf("failed to add control plane IP to default interface: %v", err)
		}
	} else {
		// default interface has already been set
		if d.DefaultIFaceName != currDefaultIFace {
			// default interface has changed, remove route and update WireguardDaemon
			fmt.Println("default interface has changed; behavior TODO")
		}
	}

	// set ip addresses on the wireguard network interface
	interfaceAddressPrefix := wgConf.ClientIp + wgConf.ClientCidr
	assignInterfaceAddr(d.InterfaceName, interfaceAddressPrefix, wgConf.ClientIp)

	// create wgctrl client to control wireguard device
	client, err := wgctrl.New()
	if err != nil {
		log.Fatalf("failed to open client: %v", err)
	}
	defer client.Close()

	// TODO: DEBUG
	devices, err := client.Devices()
	if err != nil {
		log.Fatalf("failed to get devices: %v", err)
	}

	for _, d := range devices {
		printDevice(d)
	}
	// TODO: END DEBUG

	// parse keys into wgtypes
	pri, err := wgtypes.ParseKey(wgConf.ClientPrivateKey)
	if err != nil {
		log.Fatalf("failed to parse private key: %v", err)
	}

	pub, err := wgtypes.ParseKey(wgConf.ServerPublicKey)
	if err != nil {
		log.Fatalf("failed to parse public key: %v", err)
	}

	// configure wireguard interface with peer information
	port := 51720
	ka := 20 * time.Second

	conf := wgtypes.Config{
		PrivateKey:   &pri,
		ListenPort:   &port,
		ReplacePeers: true,
		Peers: []wgtypes.PeerConfig{{
			PublicKey:    pub,
			Remove:       false,
			UpdateOnly:   false,
			PresharedKey: nil,
			Endpoint: &net.UDPAddr{
				IP:   net.ParseIP(wgConf.ServerIp),
				Port: wgConf.ServerPort,
			},
			PersistentKeepaliveInterval: &ka,
			ReplaceAllowedIPs:           true,
			AllowedIPs: []net.IPNet{{
				IP:   net.ParseIP("0.0.0.0"),
				Mask: net.IPv4Mask(0, 0, 0, 0),
			}},
		}},
	}

	if err := client.ConfigureDevice(d.InterfaceName, conf); err != nil {
		if os.IsNotExist(err) {
			fmt.Println(err)
		} else {
			log.Fatalf("Unknown config error: %v", err)
		}
	}

	// now that wireguard tunnel is fully up, we add the routes to the system to redirect traffic there

	// set routes to be the de-duped peer allowed IPs (routable subnets)
	// TODO: enable this to be overridden for client via cli flag
	// TODO: this needs to actually be the de-duped peer allowed IPs

	// we create a slightly more specific than default route
	routes := []routeInfo{{
		ipAddressPrefix: "0.0.0.0/1",
		interfaceName:   d.InterfaceName,
	}, {
		ipAddressPrefix: "128.0.0.0/1",
		interfaceName:   d.InterfaceName,
	}}

	// add peer routes to the rvpn wireguard interface
	for _, newRoute := range routes {
		err = routeAddIFace(newRoute.ipAddressPrefix, newRoute.interfaceName)
		if err != nil {
			log.Fatalf("something went wrong with route add: %v", err)
		}
	}

	d.appendedRoutes = routes
}

// Disconnect instructs the wireguard daemon to disconnect from current connection
func (d *WireguardDaemon) Disconnect() {
	log.Println("disconnecting rVPN daemon...")

	// bring device down
	err := d.Device.Down()
	if err != nil {
		log.Fatalf("failed to shut down device")
	}

	// clean up appended routing rules (routes to send to VPN)
	for _, appendedRoute := range d.appendedRoutes {
		err := routeDelIFace(appendedRoute.ipAddressPrefix, appendedRoute.interfaceName)
		if err != nil {
			// this should not fatal
			log.Printf("warn: failed to delete appended route: %v", err)
		}
	}

	d.appendedRoutes = []routeInfo{}
}

// ShutdownDevice shuts down the wireguard device
func (d *WireguardDaemon) ShutdownDevice() {
	log.Println("shutting down rVPN daemon...")

	// clean up listener and device
	d.Uapi.Close()
	d.Device.Close()

	// clean up routes on default interface - remove server ip from default interface
	if d.DefaultIFaceName != "" {
		err := routeDelGateway(d.ServerIP.String(), d.DefaultGatewayIP)
		if err != nil {
			log.Fatalf("warn: failed to delete server ip from default interface: %v", err)
		}

		// remove control plane ip from default interface
		err = routeDelGateway(d.ControlPlaneIP.String(), d.DefaultGatewayIP)
		if err != nil {
			log.Fatalf("warn: failed to delete control plane ip from default interface: %v", err)
		}
	}

	// clean up appended routing rules (routes to send to VPN)
	for _, appendedRoute := range d.appendedRoutes {
		err := routeDelIFace(appendedRoute.ipAddressPrefix, appendedRoute.interfaceName)
		if err != nil {
			// this should not fatal as route may already be deleted
			log.Printf("warn: failed to delete appended route: %v", err)
		}
	}
}
