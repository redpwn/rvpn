//go:build linux

package wg

import (
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"
	"time"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type WireguardDaemon struct {
	Device           *device.Device
	Uapi             net.Listener
	DefaultIFaceLink netlink.Link
	ServerIP         netip.Prefix
	ControlPlaneIP   netip.Prefix
	InterfaceName    string

	// internal variables used for managing the daemon
	appendedRoutes    []netlink.Route // routes stashed outside of rvpn
	appendedSrcRules  []*netlink.Rule // rules for source routing
	appendedSrcRoutes []netlink.Route // routes for source routing
	vpnServerMode     bool
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

// UpdateClientConf updates the configuration of a WireguardDaemon with the provided config for rVPN clients
// NOTE: must route controlPlaneAddr to default gateway to preserve control plane WebSocket
func (d *WireguardDaemon) UpdateClientConf(wgConf ClientWgConfig, controlPlaneAddr string) {
	log.Println("starting wireguard network interface configuration for clients")

	interfaceLink, err := netlink.LinkByName(d.InterfaceName)
	if err != nil {
		log.Fatalf("failed to get rvpn wireguard interface link: %v", err)
	}

	// bring the netlink interface up
	if err := netlink.LinkSetUp(interfaceLink); err != nil {
		log.Fatalf("failed to bring the rvpn wireguard netlink interface link up: %v", err)
	}

	err = d.Device.Up()
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

	if d.DefaultIFaceLink == nil {
		log.Println("updating default interface...")
		// default interface has not yet been defined
		d.DefaultIFaceLink = currDefaultIFace

		// set target server route on default interface as it has not been set yet
		_, parsedIPNet, err := net.ParseCIDR(d.ServerIP.String())
		if err != nil {
			log.Fatalf("failed to parse server IP into net.IPNet")
		}

		route := netlink.Route{
			LinkIndex: d.DefaultIFaceLink.Attrs().Index,
			Dst:       parsedIPNet,
			Gw:        currDefaultGateway,
		}

		if err := netlink.RouteAdd(&route); err != nil {
			log.Fatalf("failed to add server IP to default interface: %v", err)
		}

		// set control plane route on default interface
		_, parsedIPNet, err = net.ParseCIDR(d.ControlPlaneIP.String())
		if err != nil {
			log.Fatalf("failed to parse control plane IP into net.IPNet")
		}

		route = netlink.Route{
			LinkIndex: d.DefaultIFaceLink.Attrs().Index,
			Dst:       parsedIPNet,
			Gw:        currDefaultGateway,
		}

		if err := netlink.RouteAdd(&route); err != nil {
			log.Fatalf("failed to add control plane IP to default interface: %v", err)
		}
	} else {
		// default interface has already been set
		if d.DefaultIFaceLink.Attrs().Name != currDefaultIFace.Attrs().Name {
			// default interface has changed, remove route and update WireguardDaemon
			fmt.Println("default interface has changed; behavior TODO")
		}
	}

	// flush routes on the rvpn wireguard interface
	interfaceRoutes, err := netlink.RouteList(interfaceLink, unix.AF_INET)
	if err != nil {
		log.Fatalf("failed to get route list for rvpn wireguard interface")
	}

	for _, interfaceRoute := range interfaceRoutes {
		if err := netlink.RouteDel(&interfaceRoute); err != nil {
			log.Fatalf("failed to delete route from rvpn wireguard interface: %v", err)
		}
	}

	// enable routing by source ip for default interface
	// NOTE: this is so reply traffic to defualt interface exits directly through default interface
	// even if there is a new default route which the traffic should go through
	err = d.enableSourceRouting(currDefaultIFace, currDefaultGateway)
	if err != nil {
		log.Fatalf("something went wrong when enabling source routing: %v", err)
	}

	// set ip addresses on the wireguard network interface
	interfaceAddressPrefix := wgConf.ClientIp + wgConf.ClientCidr
	assignInterfaceAddr(d.InterfaceName, interfaceAddressPrefix)

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
	_, parsedPeerAllowedIP, err := net.ParseCIDR("0.0.0.0/1") // we create a slightly more specific than default route
	if err != nil {
		log.Fatalf("failed to parse peer allowed IP into net.IPNet")
	}
	routes := []netlink.Route{{
		LinkIndex: interfaceLink.Attrs().Index,
		Dst:       parsedPeerAllowedIP,
	}}

	// add peer routes to the rvpn wireguard interface
	for _, newRoute := range routes {
		if err := netlink.RouteAdd(&newRoute); err != nil {
			log.Fatalf("failed to add new route to rvpn wireguard interface: %v", err)
		}
	}

	d.appendedRoutes = routes

	// tell the daemon that we are in client mode
	d.vpnServerMode = false
}

// UpdateServeConf updates the configuration of a WireguardDaemon with the provided config for rVPN VPN serving clients
func (d *WireguardDaemon) UpdateServeConf(wgConf ServeWgConfig) {
	log.Println("starting wireguard network interface configuration for serving")

	interfaceLink, err := netlink.LinkByName("rvpn0")
	if err != nil {
		log.Fatalf("failed to get rvpn wireguard interface link: %v", err)
	}

	// bring the netlink interface up
	if err := netlink.LinkSetUp(interfaceLink); err != nil {
		log.Fatalf("failed to bring the rvpn wireguard netlink interface link up: %v", err)
	}

	err = d.Device.Up()
	if err != nil {
		log.Fatalf("failed to bring up device: %v", err)
	}

	// find default adapater
	currDefaultIFace, _, err := findDefaultInterface()
	if err != nil {
		log.Fatalf("failed to find default interface: %v", err)
	}

	if d.DefaultIFaceLink == nil {
		log.Println("updating default interface...")
		// default interface has not yet been defined
		d.DefaultIFaceLink = currDefaultIFace
	} else {
		// default interface has already been set
		if d.DefaultIFaceLink.Attrs().Name != currDefaultIFace.Attrs().Name {
			// default interface has changed, remove route and update WireguardDaemon
			fmt.Println("default interface has changed; behavior TODO")
		}
	}

	// set ip addresses on the wireguard network interface
	interfaceAddressPrefix := wgConf.InternalIp + wgConf.InternalCidr
	assignInterfaceAddr(d.InterfaceName, interfaceAddressPrefix)

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
	pri, err := wgtypes.ParseKey(wgConf.PrivateKey)
	if err != nil {
		log.Fatalf("failed to parse private key: %v", err)
	}

	// configure wireguard interface with peer information
	port := wgConf.ListenPort
	peers := []wgtypes.PeerConfig{}

	for _, clientPeer := range wgConf.Peers {
		parsedPubkey, err := wgtypes.ParseKey(clientPeer.PublicKey)
		if err != nil {
			// log failure but continue
			log.Printf("failed to parse peer pubkey: %v", err)
		}

		wgPeer := wgtypes.PeerConfig{
			PublicKey:         parsedPubkey,
			Remove:            false,
			UpdateOnly:        false,
			PresharedKey:      nil,
			Endpoint:          nil,
			ReplaceAllowedIPs: true,
			AllowedIPs: []net.IPNet{{
				IP:   net.ParseIP(clientPeer.AllowedIP),
				Mask: net.IPv4Mask(255, 255, 255, 255), // TODO: actually use clientPeer.AllowedCidr
			}},
		}

		peers = append(peers, wgPeer)
	}

	conf := wgtypes.Config{
		PrivateKey:   &pri,
		ListenPort:   &port,
		ReplacePeers: true,
		Peers:        peers,
	}

	if err := client.ConfigureDevice(d.InterfaceName, conf); err != nil {
		if os.IsNotExist(err) {
			fmt.Println(err)
		} else {
			log.Fatalf("Unknown config error: %v", err)
		}
	}

	// configure forwarding rules (via iptables for now but consider netfilter)
	err = d.enableForwarding()
	if err != nil {
		log.Printf("failed to enable forwarding: %v", err)
	}

	d.vpnServerMode = true
}

// AppendPeers appends a new client peer to the rVPN Wireguard configuration
func (d *WireguardDaemon) AppendPeers(toAppendPeers []WireGuardPeer) {
	log.Printf("appending new peers to Wireguard Daemon")

	// create wgctrl client to control wireguard device
	client, err := wgctrl.New()
	if err != nil {
		log.Fatalf("failed to open client: %v", err)
	}
	defer client.Close()

	// configure wireguard interface with peer information
	peers := []wgtypes.PeerConfig{}

	for _, clientPeer := range toAppendPeers {
		parsedPubkey, err := wgtypes.ParseKey(clientPeer.PublicKey)
		if err != nil {
			// log failure but continue
			log.Printf("failed to parse peer pubkey: %v", err)
		}

		wgPeer := wgtypes.PeerConfig{
			PublicKey:         parsedPubkey,
			Remove:            false,
			UpdateOnly:        false,
			PresharedKey:      nil,
			Endpoint:          nil,
			ReplaceAllowedIPs: false, // TODO: investigate more about this setting
			AllowedIPs: []net.IPNet{{
				IP:   net.ParseIP(clientPeer.AllowedIP),
				Mask: net.IPv4Mask(255, 255, 255, 255), // TODO: actually use clientPeer.AllowedCidr
			}},
		}

		peers = append(peers, wgPeer)
	}

	conf := wgtypes.Config{
		ReplacePeers: false,
		Peers:        peers,
	}

	if err := client.ConfigureDevice(d.InterfaceName, conf); err != nil {
		if os.IsNotExist(err) {
			fmt.Println(err)
		} else {
			log.Fatalf("Unknown config error: %v", err)
		}
	}
}

// TODO: function to remove peer from Wireguard configuration

// Disconnect instructs the wireguard daemon to disconnect from current connection
func (d *WireguardDaemon) Disconnect() {
	err := d.Device.Down()
	if err != nil {
		log.Fatalf("failed to shut down device")
	}

	// if vpnServerMode is true, delete iptables rules for forwarding
	if d.vpnServerMode {
		err = d.disableForwarding()
		if err != nil {
			log.Printf("failed to disable forwarding: %v", err)
		}
	}

	// cleanup any routing rules
	for _, appendedRoute := range d.appendedRoutes {
		if err := netlink.RouteDel(&appendedRoute); err != nil {
			log.Printf("failed to delete appended route: %v", err)
		}
	}

	d.appendedRoutes = []netlink.Route{}

	// cleanup source routing rules and routes
	d.stopSourceRouting()
}

// ShutdownDevice shuts down the wireguard device
func (d *WireguardDaemon) ShutdownDevice() {
	log.Println("shutting down rVPN daemon...")

	// clean up listener and device
	d.Uapi.Close()
	d.Device.Close()

	// clean up routes - remove server ip from default interface
	_, parsedIPNet, err := net.ParseCIDR(d.ServerIP.String())
	if err != nil {
		log.Fatalf("failed to parse server IP into net.IPNet")
	}

	route := netlink.Route{
		LinkIndex: d.DefaultIFaceLink.Attrs().Index,
		Dst:       parsedIPNet,
	}

	if err := netlink.RouteDel(&route); err != nil {
		log.Fatalf("failed to delete server IP from default interface: %v", err)
	}

	// remote control plane ip from default interface
	_, parsedIPNet, err = net.ParseCIDR(d.ControlPlaneIP.String())
	if err != nil {
		log.Fatalf("failed to parse server IP into net.IPNet")
	}

	route = netlink.Route{
		LinkIndex: d.DefaultIFaceLink.Attrs().Index,
		Dst:       parsedIPNet,
	}

	if err := netlink.RouteDel(&route); err != nil {
		log.Fatalf("failed to delete control plane IP from default interface: %v", err)
	}

	// if vpnServerMode is true, delete iptables rules for forwarding
	if d.vpnServerMode {
		err = d.disableForwarding()
		if err != nil {
			log.Printf("failed to disable forwarding: %v", err)
		}
	}

	// cleanup any appended routing rules
	for _, appendedRoute := range d.appendedRoutes {
		if err := netlink.RouteDel(&appendedRoute); err != nil {
			log.Printf("failed to delete appended route: %v", err)
		}
	}

	// cleanup source routing rules and routes
	d.stopSourceRouting()
}
