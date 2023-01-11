package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/rpc"
	"os"

	"github.com/denisbrodbeck/machineid"
	"github.com/redpwn/rvpn/common"
	"github.com/redpwn/rvpn/daemon"
)

// client.go holds functions which interact (connect, disconnect, status) with the client daemon via rpc

// getControlPanelAuthToken gets the control panel auth token from state
func getControlPanelAuthToken() string {
	rVPNState, err := common.GetRVpnState()
	if err != nil {
		fmt.Printf("failed to get rVPN state: %v\n", err)
		os.Exit(1)
	}

	return rVPNState.ControlPlaneAuth
}

// ControlPanelAuthLogin saves the given token as the login token
func ControlPanelAuthLogin(token string) {
	rVPNState, err := common.GetRVpnState()
	if err != nil {
		fmt.Printf("failed to get rVPN state: %v\n", err)
		os.Exit(1)
	}

	rVPNState.ControlPlaneAuth = token
	err = common.SetRVpnState(rVPNState)
	if err != nil {
		fmt.Println("failed to save rVPN state")
		os.Exit(1)
	}

	fmt.Println("successfully set rVPN login token!")
}

// ClientConnectProfile instructs the rVPN daemon to connect to a target via rpc
func ClientConnectProfile(profile string, opts common.ClientOptions) {
	// connect to rVPN daemon
	client, err := rpc.Dial("tcp", "127.0.0.1:52370")
	if err != nil {
		fmt.Println("failed to connect to rVPN daemon", err)
		os.Exit(1)
	}
	defer client.Close()

	// ensure device is not already connected
	var connectionStatus daemon.RVPNStatus
	err = client.Call("RVPNDaemon.Status", "", &connectionStatus)
	if err != nil {
		fmt.Println("failed to get rVPN status", err)
		os.Exit(1)
	}

	if connectionStatus != daemon.StatusDisconnected {
		// device is already connected, early exit
		fmt.Println("device is already connected to a rVPN target, disconnect and try again")
		os.Exit(1)
	}

	// ensure device is registered for target
	controlPanelAuthToken := getControlPanelAuthToken()
	if controlPanelAuthToken == "" {
		fmt.Println(`not logged into rVPN, login first using "rvpn login [token]"`)
		os.Exit(1)
	}

	machineId, err := machineid.ID()
	if err != nil {
		fmt.Println("failed to get machine id", err)
		os.Exit(1)
	}

	controlPlaneURL := RVPN_CONTROL_PLANE + "/api/v1/target/" + profile + "/register_device"
	jsonStr := []byte(fmt.Sprintf(`{"hardwareId":"%s"}`, machineId))

	req, err := http.NewRequest("POST", controlPlaneURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		fmt.Println("register device request failed", err)
		os.Exit(1)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", controlPanelAuthToken))
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Println("failed to send device registration request", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		fmt.Println("invalid rVPN login token, please check target / login token and try again")
		os.Exit(1)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("failed to read device registration response", err)
		os.Exit(1)
	}

	deviceRegistrationResp := common.RegisterDeviceResponse{}
	err = json.Unmarshal(body, &deviceRegistrationResp)
	if err != nil {
		fmt.Println("failed to unmarshal device registration response", err)
		os.Exit(1)
	}

	// start connection by issuing request to rVPN daemon
	connectionRequest := daemon.ConnectRequest{
		Profile:        profile,
		DeviceToken:    deviceRegistrationResp.DeviceToken,
		ControlPlaneWS: RVPN_CONTROL_PLANE_WS,
		Opts:           opts,
	}

	var connectionSuccess bool
	err = client.Call("RVPNDaemon.Connect", connectionRequest, &connectionSuccess)
	if err != nil {
		fmt.Printf("failed to connect rVPN target: %v", err)
		os.Exit(1)
	}

	fmt.Printf("rVPN successfully connected to profile %s\n", profile)
}

// ClientDisconnectProfile instructs the rVPN daemon to disconnect from the current target via rpc
func ClientDisconnectProfile() {
	client, err := rpc.Dial("tcp", "127.0.0.1:52370")
	if err != nil {
		fmt.Println("failed to connect to rVPN daemon", err)
		os.Exit(1)
	}
	defer client.Close()

	// ensure device is connected
	var connectionStatus daemon.RVPNStatus
	err = client.Call("RVPNDaemon.Status", "", &connectionStatus)
	if err != nil {
		fmt.Println("failed to get rVPN status", err)
		os.Exit(1)
	}

	if connectionStatus == daemon.StatusDisconnected {
		// device is already connected, early exit
		fmt.Println("device is not connected to a rVPN target, disconnect and try again")
		os.Exit(1)
	}

	var disconnectSuccess bool
	err = client.Call("RVPNDaemon.Disconnect", "", &disconnectSuccess)
	if err != nil {
		fmt.Println("failed to disconnect from rVPN connection", err)
		os.Exit(1)
	}

	fmt.Println("successfully disconnected rVPN")
}

// ClientStatus gets the status of the rVPN daemon via rpc
func ClientStatus() {
	client, err := rpc.Dial("tcp", "127.0.0.1:52370")
	if err != nil {
		fmt.Println("failed to connect to rVPN daemon")
		os.Exit(1)
	}
	defer client.Close()

	var rVPNState daemon.RVPNStatus
	err = client.Call("RVPNDaemon.Status", "", &rVPNState)
	if err != nil {
		fmt.Println("failed to get status from rVPN daemon", err)
		os.Exit(1)
	}

	if rVPNState == daemon.StatusConnected {
		fmt.Println("rVPN is currently connected to a profile")
	} else if rVPNState == daemon.StatusDisconnected {
		fmt.Println("rVPN is not currently connected to a profile")
	} else if rVPNState == daemon.StatusServing {
		fmt.Println("rVPN is currently serving as a target VPN server")
	} else {
		fmt.Println("something went wrong, rVPN status is unrecognized")
	}
}
