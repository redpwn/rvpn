//go:build windows

package wg

import (
	"fmt"
	"log"
	"net"
	"net/netip"
	"os"
	"time"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/ipc"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

type WireguardDaemon struct {
	Device         *device.Device
	Uapi           net.Listener
	Adapter        *winipcfg.IPAdapterAddresses
	DefaultAdapter *winipcfg.IPAdapterAddresses
	DefaultGateway netip.Addr
	ServerIP       netip.Prefix
	ControlPlaneIP netip.Prefix
	InterfaceName  string

	// internal variables used for managing the daemon
	prevRoutes []*winipcfg.RouteData
}

// NewWireguardDaemon returns a new WireguardDaemon NOTE: this is uninitialized
func NewWireguardDaemon() *WireguardDaemon {
	return &WireguardDaemon{
		prevRoutes: []*winipcfg.RouteData{},
	}
}

// StartDevice starts the wireguard networking interface used by rVPN
func (d *WireguardDaemon) StartDevice(errs chan error) {
	// use wireguard to create a new device
	interfaceName := "rvpn0"
	family := winipcfg.AddressFamily(windows.AF_INET) // TODO: investigate how we differentiate between AF_INET and AF_INET6 for ipv4 vs ipv6
	deviceMTU := 1420

	// open TUN device
	tun, err := tun.CreateTUN(interfaceName, deviceMTU)
	if err != nil {
		log.Fatalf("failed to create TUN device: %v", err)
	}

	// this interface name is the friendly name
	realInterfaceName, err := tun.Name()
	if err != nil {
		log.Println("failed to get interface name")
	}

	interfaceName = realInterfaceName

	// get the adapter using the friendly interfaceName
	out, err := winipcfg.GetAdaptersAddresses(windows.AF_UNSPEC, winipcfg.GAAFlagDefault)
	if err != nil {
		log.Fatalf("failed to get adapter addresses: %v", err)
	}

	found := false
	for _, adapter := range out {
		if adapter.FriendlyName() == interfaceName {
			// this ipAdapterAddress structure is the header node for our target adapter
			d.Adapter = adapter
			found = true
		}
	}

	if !found {
		log.Fatalf("failed to get adapter for TUN device")
	}

	// set MTU on the adapter because wireguard-go createTUN() does not respect MTU
	adapterInteface, err := d.Adapter.LUID.IPInterface(family)
	if err != nil {
		log.Fatalf("failed to get adapter interface: %v", err)
	}

	adapterInteface.NLMTU = uint32(deviceMTU)
	adapterInteface.Set()

	log.Printf("set interface MTU to: %d", deviceMTU)

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

	uapi, err := ipc.UAPIListen(interfaceName)
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
// NOTE: must route controlPlaneAddr to default gateway to preserve control plane WebSocket
func (d *WireguardDaemon) UpdateClientConf(wgConf ClientWgConfig, controlPlaneAddr string) {
	log.Println("starting wireguard network interface configuration")

	err := d.Device.Up()
	if err != nil {
		log.Fatalf("failed to bring up device: %v", err)
	}

	family := winipcfg.AddressFamily(windows.AF_INET) // TODO: investigate how we differentiate between AF_INET and AF_INET6 for ipv4 vs ipv6

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
	currDefaultInterface, gatewayIP, err := findDefaultLUID(family, d.Adapter.LUID)
	if err != nil {
		log.Fatalf("failed to find default interface: %v", err)
	}

	out, err := winipcfg.GetAdaptersAddresses(windows.AF_UNSPEC, winipcfg.GAAFlagDefault)
	if err != nil {
		log.Fatalf("failed to get adapter addresses: %v", err)
	}

	var currDefaultAdapter *winipcfg.IPAdapterAddresses
	for _, adapter := range out {
		if adapter.LUID == currDefaultInterface {
			currDefaultAdapter = adapter
			fmt.Println("found current default network adapter")
		}
	}

	if d.DefaultAdapter == nil {
		log.Println("updating default interface...")
		// default interface has not yet been defined
		d.DefaultAdapter = currDefaultAdapter

		// set target server route on default adapter as it has not yet been set
		d.DefaultAdapter.LUID.AddRoute(d.ServerIP, gatewayIP, 0)
		// set control plane address route on default adapater
		d.DefaultAdapter.LUID.AddRoute(d.ControlPlaneIP, gatewayIP, 0)

		d.DefaultGateway = gatewayIP
	} else {
		// defaultInterface has already been set
		if d.DefaultAdapter.LUID != currDefaultInterface {
			// default interface has changed, remove route and update WireguardDaemon
			fmt.Println("default interface has changed; behavior TODO")
		}
	}

	// set routes to be the de-duped peer allowed IPs # TODO: right now we just hardcode to all traffic
	interfaceIP := netip.MustParsePrefix(wgConf.ClientIp + wgConf.ClientCidr)
	interfaceIPs := []netip.Prefix{interfaceIP}
	peerAllowedIP := netip.MustParsePrefix("0.0.0.0/0") // TODO: this needs to actually be the de-duped peer allowed IPs
	routes := []*winipcfg.RouteData{{
		Destination: peerAllowedIP.Masked(),
		NextHop:     netip.IPv4Unspecified(),
		Metric:      0,
	}}

	// NOTE: LUID.FlushRoutes is broken, so we manually track previous routes and delete them
	for _, prevRoute := range d.prevRoutes {
		err = d.Adapter.LUID.DeleteRoute(prevRoute.Destination, prevRoute.NextHop)
		if err != nil {
			log.Fatalf("failed to delete route: %v", err)
		}
	}

	// add peer routes to the rvpn wireguard interface
	for _, newRoute := range routes {
		err = d.Adapter.LUID.AddRoute(newRoute.Destination, newRoute.NextHop, newRoute.Metric)
		if err != nil {
			log.Fatalf("failed to set routes on interface: %v", err)
		}
	}

	d.prevRoutes = routes

	// set ip address of rvpn wireguard interface
	err = d.Adapter.LUID.SetIPAddressesForFamily(family, interfaceIPs)
	if err != nil {
		log.Fatalf("failed to set ip address on interface: %v", err)
	}

	// set DNS on the wireguard interface
	err = d.Adapter.LUID.SetDNS(family, []netip.Addr{netip.MustParseAddr("1.1.1.1")}, []string{})
	if err != nil {
		log.Fatalf("failed to set DNS on interface: %v", err)
	}

	log.Println("finished wireguard network interface configuration")

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

	pri, err := wgtypes.ParseKey(wgConf.ClientPrivateKey)
	if err != nil {
		log.Fatalf("failed to parse private key: %v", err)
	}

	pub, err := wgtypes.ParseKey(wgConf.ServerPublicKey)
	if err != nil {
		log.Fatalf("failed to parse public key: %v", err)
	}

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
}

// Disconnect instructs the wireguard daemon to disconnect from current connection
func (d *WireguardDaemon) Disconnect() {
	err := d.Device.Down()
	if err != nil {
		log.Fatalf("failed to shut down device")
	}

	// NOTE: LUID.FlushRoutes is broken, so we manually track previous routes and delete them
	for _, prevRoute := range d.prevRoutes {
		err = d.Adapter.LUID.DeleteRoute(prevRoute.Destination, prevRoute.NextHop)
		if err != nil {
			log.Printf("failed to delete route: %v", err)
		}
	}

	d.prevRoutes = []*winipcfg.RouteData{}
}

// ShutdownDevice shuts down the wireguard device
func (d *WireguardDaemon) ShutdownDevice() {
	log.Println("shutting down windows rVPN daemon...")

	// clean up listener and device
	d.Uapi.Close()
	d.Device.Close()

	// clean up routes (route we added on default interface to let wg traffic route properly)
	d.DefaultAdapter.LUID.DeleteRoute(d.ServerIP, d.DefaultGateway)
	d.DefaultAdapter.LUID.DeleteRoute(d.ControlPlaneIP, d.DefaultGateway)
}
