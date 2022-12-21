//go:build linux

package wg

import (
	"log"
	"os"

	"github.com/vishvananda/netlink"
)

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
