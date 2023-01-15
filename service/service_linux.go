//go:build linux

package service

import "github.com/redpwn/rvpn/daemon"

// StartRVPNDaemon is the entrypoint to starting the linux rVPN daemon
// NOTEL: cliClientPath is a stubbed param and has no effect
func StartRVPNDaemon(cliClientPath string) {
	newDaemon := daemon.NewRVPNDaemon()
	newDaemon.Start()
}
