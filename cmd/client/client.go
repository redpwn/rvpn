package main

import (
	"fmt"
	"net/rpc"
	"os"
)

// client.go holds functions which interact (connect, disconnect, status) with the client daemon via rpc

// ClientConnectProfile instructs the rVPN daemon to connect to a target via rpc
func ClientConnectProfile(profile string) {
	client, err := rpc.Dial("tcp", "127.0.0.1:52370")
	if err != nil {
		fmt.Println("failed to connect to rVPN daemon")
		os.Exit(1)
	}

	connectionRequest := ConnectRequest{
		Profile:     "test",
		DeviceToken: "tmp",
	}

	var connectionSuccess bool
	err = client.Call("RVPNDaemon.Connect", connectionRequest, &connectionSuccess)
	if err != nil {
		fmt.Println("failed to connect rVPN target", err)
		os.Exit(1)
	}
}

// ClientDisconnectProfile instructs the rVPN daemon to disconnect from the current target via rpc
func ClientDisconnectProfile() {
	client, err := rpc.Dial("tcp", "127.0.0.1:52370")
	if err != nil {
		fmt.Println("failed to connect to rVPN daemon")
		os.Exit(1)
	}

	var disconnectSuccess bool
	err = client.Call("RVPNDaemon.Disconnect", "", &disconnectSuccess)
	if err != nil {
		fmt.Println("failed to disconnect from rVPN connection", err)
		os.Exit(1)
	}
}

// ClientStatus gets the status of the rVPN daemon via rpc
func ClientStatus() {
	client, err := rpc.Dial("tcp", "127.0.0.1:52370")
	if err != nil {
		fmt.Println("failed to connect to rVPN daemon")
		os.Exit(1)
	}

	var rVPNState RVPNStatus
	err = client.Call("RVPNDaemon.Status", "", &rVPNState)
	if err != nil {
		fmt.Println("failed to get status from rVPN daemon", err)
		os.Exit(1)
	}

	fmt.Println(rVPNState)
}
