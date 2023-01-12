//go:build linux

package service

// EnsureServiceStarted ensures the rVPN daemon has been started
func EnsureServiceStarted() error {
	return nil
}

func StartRVPNDaemon() {
	// TODO: implement starting the linux daemon
}
