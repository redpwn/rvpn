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
// functions in this file assume the daemon is running otherwise they will error

// getControlPanelAuthToken gets the control panel auth token from state
func getControlPanelAuthToken(client *rpc.Client) string {
	rVPNState, err := GetRVpnState(client)
	if err != nil {
		fmt.Printf("failed to get rVPN state: %v\n", err)
		os.Exit(1)
	}

	return rVPNState.ControlPlaneAuth
}

// ControlPanelAuthLogin saves the given token as the login token
func ControlPanelAuthLogin(token string) {
	client, err := rpc.Dial("tcp", "127.0.0.1:52370")
	if err != nil {
		fmt.Println("failed to connect to rVPN daemon", err)
		os.Exit(1)
	}
	defer client.Close()

	rVPNState, err := GetRVpnState(client)
	if err != nil {
		fmt.Printf("failed to get rVPN state: %v\n", err)
		os.Exit(1)
	}

	rVPNState.ControlPlaneAuth = token
	err = SetRVpnState(client, rVPNState)
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
	controlPanelAuthToken := getControlPanelAuthToken(client)
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

	// either receive message from connectEvent chan or timeout 5 seconds
	// if connectEvent then test for connectivity by pinging the VPN server

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

type targetProfileInfo struct {
	Name string `json:"name"`
}

// ListTargetProfiles lists the available targets for the logged in user to connect to
func ListTargetProfiles() {
	client, err := rpc.Dial("tcp", "127.0.0.1:52370")
	if err != nil {
		fmt.Println("failed to connect to rVPN daemon")
		os.Exit(1)
	}
	defer client.Close()

	// get control plane authentication token
	rVPNState, err := GetRVpnState(client)
	if err != nil {
		fmt.Println("failed to get rVPN state")
		os.Exit(1)
	}

	controlPanelAuthToken := rVPNState.ControlPlaneAuth
	if controlPanelAuthToken == "" {
		fmt.Println(`not logged into rVPN, login first"`)
		os.Exit(1)
	}

	controlPlaneURL := RVPN_CONTROL_PLANE + "/api/v1/target/"

	req, err := http.NewRequest("GET", controlPlaneURL, nil)
	if err != nil {
		fmt.Println("failed to create request to list targets")
		os.Exit(1)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", controlPanelAuthToken))
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		fmt.Println("failed to send request to list targets")
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		fmt.Println("invalid rVPN login token, please check target / login token and try again")
		os.Exit(1)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("failed to read user targets response: %w", err)
		os.Exit(1)
	}

	var profileList []targetProfileInfo

	err = json.Unmarshal(body, &profileList)
	if err != nil {
		fmt.Println("failed to read message from control plane")
		os.Exit(1)
	}

	// print resulting target list
	fmt.Println("available rVPN target profiles:")
	for _, targetInfo := range profileList {
		fmt.Println(targetInfo.Name)
	}
}

// EnsureDaemonStarted checks if the daemon is started, if not it prompts to start the daemon
func EnsureDaemonStarted() {
	// TODO: determine if elevating and running the command is neccesary per UX
	client, err := rpc.Dial("tcp", "127.0.0.1:52370")
	if err != nil {
		fmt.Println("failed to connect to rVPN daemon, ensure the rvpn service is running")
		os.Exit(1)
	}
	defer client.Close()

	// ping to ensure daemon is alive
	var pingStatus bool
	err = client.Call("RVPNDaemon.Ping", "", &pingStatus)
	if err != nil {
		fmt.Println("failed to connect to rVPN daemon, ensure the rvpn service is running")
		os.Exit(1)
	}

	if !pingStatus {
		fmt.Println("failed to connect to rVPN daemon, ensure the rvpn service is running")
		os.Exit(1)
	}
}
