//go:build darwin

package main

import "fmt"

// ClientServeProfile instructs the rVPN daemon to serve as a VPN server for a target via rpc
func ClientServeProfile(profile string) {
	// NOTE: it is currently not supported for the windows client to serve as a VPN server
	fmt.Println("ERROR: darwin rVPN client is not supported to serve as a VPN server")
}
