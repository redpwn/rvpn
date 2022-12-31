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
	InterfaceName    string

	// internal variables used for managing the daemon
	stashedRoutes []netlink.Route // routes stashed outside of rvpn
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

	serverIP, err := netip.ParsePrefix(wgConf.ServerIp + "/32")
	if err != nil {
		log.Fatalf("failed to parse server ip: %v", err)
	}

	d.ServerIP = serverIP

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
	} else {
		// default interface has already been set
		if d.DefaultIFaceLink.Attrs().Name != currDefaultIFace.Attrs().Name {
			// default interface has changed, remove route and update WireguardDaemon
			fmt.Println("default interface has changed; behavior TODO")
		}
	}

	// set routes to be the de-duped peer allowed IPs
	peerAllowedIP := netip.MustParsePrefix("104.18.114.97/32") // TODO: this needs to actually be the de-duped peer allowed IPs
	_, parsedPeerAllowedIP, err := net.ParseCIDR(peerAllowedIP.String())
	if err != nil {
		log.Fatalf("failed to parse peer allowed IP into net.IPNet")
	}
	routes := []netlink.Route{{
		LinkIndex: interfaceLink.Attrs().Index,
		Dst:       parsedPeerAllowedIP,
	}}

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

	// re-apply routing rules from stashed overlapping routes
	for _, stashedRoute := range d.stashedRoutes {
		if err := netlink.RouteAdd(&stashedRoute); err != nil {
			log.Fatalf("failed to add back stashed route: %v", err)
		}
	}

	// stash routes and delete which overlap exactly with peer allowed ips
	overlappingRoutes, err := findOverlappingRoutes(routes)
	if err != nil {
		log.Fatalf("failed to find overlapping routes between current routes and peer routes")
	}

	for _, overlappingRoute := range overlappingRoutes {
		if err := netlink.RouteDel(&overlappingRoute); err != nil {
			log.Fatalf("failed to delete overlapping route: %v", err)
		}
	}

	d.stashedRoutes = overlappingRoutes

	// add peer routes to the rvpn wireguard interface
	for _, newRoute := range routes {
		if err := netlink.RouteAdd(&newRoute); err != nil {
			log.Fatalf("failed to add new route to rvpn wireguard interface: %v", err)
		}
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
	pri, err := wgtypes.ParseKey(wgConf.PrivateKey)
	if err != nil {
		log.Fatalf("failed to parse private key: %v", err)
	}

	pub, err := wgtypes.ParseKey(wgConf.PublicKey)
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
				IP:   net.ParseIP("104.18.114.97"),     // "0.0.0.0"
				Mask: net.IPv4Mask(255, 255, 255, 255), // (0,0,0,0)
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
}

// Disconnect instructs the wireguard daemon to disconnect from current connection
func (d *WireguardDaemon) Disconnect() {
	err := d.Device.Down()
	if err != nil {
		log.Fatalf("failed to shut down device")
	}

	// TODO: cleanup any routing rules
}

// ShutdownDevice shuts down the wireguard device
func (d *WireguardDaemon) ShutdownDevice() {
	log.Println("shutting down rVPN daemon...")

	// clean up listener and device
	d.Uapi.Close()
	d.Device.Close()

	// clean up routes - remove server ip from defualt interface
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

}
