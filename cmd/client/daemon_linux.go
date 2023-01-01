//go:build linux

package main

// EnsureDaemonStarted checks if the daemon is started
func EnsureDaemonStarted() error {
	// elevate and run the command "rvpn daemon"
	// TODO: complete this function
	return nil
}

// Serve instructs the rVPN daemon to act as a target VPN server
func (r *RVPNDaemon) Serve(args ServeRequest, reply *bool) error {
	*reply = true

	return nil
}
