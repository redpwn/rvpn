//go:build windows

package wg

import (
	"fmt"
	"net/netip"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

// findDefaultLUID finds the LUID of the default interface
// source: https://github.com/WireGuard/wireguard-windows/blob/master/tunnel/mtumonitor.go
func findDefaultLUID(family winipcfg.AddressFamily, ourLUID winipcfg.LUID) (winipcfg.LUID, netip.Addr, error) {
	lowestMetric := ^uint32(0)
	luid := winipcfg.LUID(0)
	gatewayIP := netip.Addr{}

	r, err := winipcfg.GetIPForwardTable2(family)
	if err != nil {
		return luid, gatewayIP, err
	}

	for i := range r {
		if r[i].DestinationPrefix.PrefixLength != 0 || r[i].InterfaceLUID == ourLUID {
			continue
		}
		ifrow, err := r[i].InterfaceLUID.Interface()
		if err != nil || ifrow.OperStatus != winipcfg.IfOperStatusUp {
			continue
		}

		iface, err := r[i].InterfaceLUID.IPInterface(family)
		if err != nil {
			continue
		}

		if r[i].Metric+iface.Metric < lowestMetric {
			lowestMetric = r[i].Metric + iface.Metric
			luid = r[i].InterfaceLUID
			gatewayIP = r[i].NextHop.Addr()
		}
	}

	return luid, gatewayIP, err
}
