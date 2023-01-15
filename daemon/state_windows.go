//go:build windows

package daemon

// getRVpnStatePath gets the rVPN state path from the system
func getRVpnStatePath() (string, error) {
	configPaths := configdir.SystemConfig("rvpn")

	return path.Join(configPaths[0], "state.json"), nil
}
