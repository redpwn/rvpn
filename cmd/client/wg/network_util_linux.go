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
	addrList, err := netlink.AddrList(wgInterfaceLink, netlink.FAMILY_ALL)
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

// enableSourceRouting enables source routing for the default interface
func (d *WireguardDaemon) enableSourceRouting(sourceRouteIFace netlink.Link, sourceRouteGateway net.IP) error {
	// attempt to add rule to rvpn table
	sourceRoutingRules := []*netlink.Rule{}
	sourceRoutingRoutes := []netlink.Route{}
	addrList, err := netlink.AddrList(sourceRouteIFace, netlink.FAMILY_V4)
	if err != nil {
		return err
	}

	if len(addrList) > 0 {
		// for each ipv4 address we add a source routing rule
		for i, ifaceAddr := range addrList {
			// create and add source routing rule
			sourceRoutingRule := netlink.NewRule()
			targetTable := IpSourceRouteTableBaseIdx + i

			sourceRoutingRule.Priority = 1
			sourceRoutingRule.Src = &net.IPNet{
				IP:   ifaceAddr.IP,
				Mask: net.IPv4Mask(255, 255, 255, 255),
			}
			// each address needs its own table idx
			sourceRoutingRule.Table = targetTable
			sourceRoutingRules = append(sourceRoutingRules, sourceRoutingRule)

			// create and add routing rule for source routing
			_, parsedDestIp, err := net.ParseCIDR("0.0.0.0/0")
			if err != nil {
				return err
			}

			route := netlink.Route{
				LinkIndex: sourceRouteIFace.Attrs().Index,
				Dst:       parsedDestIp,
				Gw:        sourceRouteGateway,
				Table:     targetTable,
				Priority:  50,
			}
			sourceRoutingRoutes = append(sourceRoutingRoutes, route)
		}
	}

	// add all source routing rules
	for _, sourceRoutingRule := range sourceRoutingRules {
		// add source routing rule to table
		if err := netlink.RuleAdd(sourceRoutingRule); err != nil {
			return err
		}
	}

	d.appendedSrcRules = sourceRoutingRules

	// add all source routing routes
	for _, sourceRoutingRoute := range sourceRoutingRoutes {
		// add source route
		if err := netlink.RouteAdd(&sourceRoutingRoute); err != nil {
			return err
		}
	}

	d.appendedSrcRoutes = sourceRoutingRoutes

	return nil
}

// stopSourceRouting stops source routing by deleting added rules
func (d *WireguardDaemon) stopSourceRouting() error {
	// remove source routing routes
	for _, sourceRoutingRoute := range d.appendedSrcRoutes {
		// delete source route
		if err := netlink.RouteDel(&sourceRoutingRoute); err != nil {
			return err
		}
	}

	// remote source routing rules
	for _, sourceRoutingRule := range d.appendedSrcRules {
		// delete source rule
		if err := netlink.RuleDel(sourceRoutingRule); err != nil {
			return err
		}
	}
	return nil
}
