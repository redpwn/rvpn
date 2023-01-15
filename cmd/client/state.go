package main

import (
	"errors"
	"net/rpc"

	"github.com/redpwn/rvpn/daemon"
)

func GetRVpnState(client *rpc.Client) (daemon.RVpnState, error) {
	// get state from rVPN daemon
	var rVPNState daemon.RVpnState
	err := client.Call("RVPNDaemon.GetState", "", &rVPNState)
	if err != nil {
		return daemon.RVpnState{}, err
	}

	return rVPNState, nil
}

func SetRVpnState(client *rpc.Client, rVPNState daemon.RVpnState) error {
	// get state from rVPN daemon
	var rVPNSetStateSuccess bool
	err := client.Call("RVPNDaemon.SetState", rVPNState, &rVPNSetStateSuccess)
	if err != nil {
		return err
	}

	if rVPNSetStateSuccess {
		return nil
	} else {
		return errors.New("failed to set rVPN state")
	}
}
