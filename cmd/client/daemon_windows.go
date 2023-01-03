//go:build windows

package main

// EnsureDaemonStarted checks if the daemon is started
func EnsureDaemonStarted() error {
	// elevate and run the command "rvpn daemon"
	// TODO: complete this function
	return nil
}

// Serve instructs the rVPN daemon to act as a target VPN server
func (r *RVPNDaemon) Serve(args ServeRequest, reply *bool) error {
	// NOTE: serving not supported on Windows, this is just a stub
	*reply = false

	return nil
}

// serveVPNHandler handles when the control plane instructs the daemon to act as a target VPN server
func serveVPNHandler(ctx context.Context, h jrpcHandler, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	// NOTE: serving is not suppoted no Windows, this is just a stub

	conn.Reply(ctx, req.ID, common.ServeVPNResponse{
		Success: false,
	})
}
