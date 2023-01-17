//go:build darwin

package daemon

import (
	"path"
)

const stateDir = "/var/lib/rvpn"

// getRVpnStatePath gets the rVPN state path from the system
func getRVpnStatePath() (string, error) {
	return path.Join(stateDir, "state.json"), nil
}
