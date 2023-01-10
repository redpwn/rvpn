package wg

import (
	"fmt"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// printDevice prints information about a wireguard device
func printDevice(d *wgtypes.Device) {
	const f = `interface: %s (%s)
  public key: %s
  private key: (hidden)
  listening port: %d
`

	fmt.Printf(
		f,
		d.Name,
		d.Type.String(),
		d.PublicKey.String(),
		d.ListenPort)
}
