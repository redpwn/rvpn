//go:build linux

package wg

import (
	"errors"
	"log"
	"math"
	"net"
	"os"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// assignInterfaceAddr assigns an ip to an interface using netlink
func assignInterfaceAddr(ifaceName, ipAddressPrefix string) error {
	wgInterfaceLink, err := netlink.LinkByName(ifaceName)
	if err != nil {
		return err
	}

	// delete existing addresses on the interface
	addrList, err := netlink.AddrList(wgInterfaceLink, 0)
	if err != nil {
		return err
	}

	if len(addrList) > 0 {
		for _, interfaceAddress := range addrList {
			err = netlink.AddrDel(wgInterfaceLink, &interfaceAddress)
			if err != nil {
				return err
			}
		}
	}

	log.Printf("adding address %s to interface: %s", ipAddressPrefix, ifaceName)
	addr, _ := netlink.ParseAddr(ipAddressPrefix)
	err = netlink.AddrAdd(wgInterfaceLink, addr)
	if os.IsExist(err) {
		log.Printf("interface %s already has the address: %s", ifaceName, ipAddressPrefix)
	} else if err != nil {
		return err
	}

	// on linux, the link must be brought up
	err = netlink.LinkSetUp(wgInterfaceLink)
	return err
}

// findDefaultInterfaceName finds the name of the defualt interface on system
func findDefaultInterface() (netlink.Link, net.IP, error) {
	lowestMetric := math.MaxInt
	var defaultIFaceLink netlink.Link
	var defaultGateway net.IP
	foundDefaultIFace := false

	linkList, err := netlink.LinkList()
	if err != nil {
		return nil, nil, err
	}

	for _, ifaceLink := range linkList {
		// get all routes for the interface
		routeList, err := netlink.RouteList(ifaceLink, unix.AF_INET)
		if err != nil {
			return nil, nil, err
		}

		for _, route := range routeList {
			if route.Dst == nil {
				// NOTE: if Dst is nil then it is a 0.0.0.0 route (TODO: validate this)
				if route.Priority < lowestMetric {
					// this route has a lower metric / more priority
					defaultIFaceLink = ifaceLink
					defaultGateway = route.Gw
					foundDefaultIFace = true
				}
			}
		}
	}

	if foundDefaultIFace {
		return defaultIFaceLink, defaultGateway, nil
	} else {
		return nil, nil, errors.New("default interface not found")
	}
}

// findOverlappingRoutes finds overlapping routes between current system and peerRoutes ignoring specified interface
func findOverlappingRoutes(peerRoutes []netlink.Route) ([]netlink.Route, error) {
	// TODO: pass in current interface to ignore
	overlappingRoutes := []netlink.Route{}

	linkList, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}

	for _, ifaceLink := range linkList {
		// get all routes for the interface
		routeList, err := netlink.RouteList(ifaceLink, unix.AF_INET)
		if err != nil {
			return nil, err
		}

		for _, route := range routeList {
			// check route with all peer routes
			for _, peerRoute := range peerRoutes {
				// compare with all peer routes
				if peerRoute.Dst != nil && route.Dst != nil {
					// dst exists on both routes
					if peerRoute.Dst.IP.Equal(route.Dst.IP) && peerRoute.Dst.Mask.String() == route.Dst.Mask.String() {
						// ip and mask are the same, these are the same route
						overlappingRoutes = append(overlappingRoutes, route)
						break
					}
				} else if peerRoute.Dst == nil && route.Dst == nil {
					// dst does not exist on both routes (this is default gateway route)
					overlappingRoutes = append(overlappingRoutes, route)
					break
				} else if route.Dst == nil {
					// default gateway route, check if peerRoute is "0.0.0.0/0"
					if peerRoute.Dst.IP.Equal(net.ParseIP("0.0.0.0")) {
						overlappingRoutes = append(overlappingRoutes, route)
						break
					}
				}
			}
		}
	}

	return overlappingRoutes, nil
}
