//go:build linux

package service

// EnsureServiceStarted ensures the rVPN daemon has been started
func EnsureServiceStarted() error {
	return nil
}

// StartRVPNDaemon starts the rVPN daemon on linux
// NOTEL: cliClientPath is a stubbed param and has no effect
func StartRVPNDaemon(cliClientPath string) {
	// TODO: implement starting the linux daemon

}
