package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/rpc"
	"os"

	"github.com/denisbrodbeck/machineid"
)

// client.go holds functions which interact (connect, disconnect, status) with the client daemon via rpc

// registerDeviceResponse holds the response format for device registration
type registerDeviceResponse struct {
	// device id which has been assigned to the device
	DeviceId string `json:"deviceId,omitempty"`

	// device token which is signed and authenticates the device
	DeviceToken string `json:"deviceToken,omitempty"`
}

// getControlPanelAuthToken gets the control panel auth token from state
func getControlPanelAuthToken() string {
	rVPNState, err := GetRVpnState()
	if err != nil {
		fmt.Printf("failed to get rVPN state: %v\n", err)
		os.Exit(1)
	}

	return rVPNState.ControlPlaneAuth
}

// ControlPanelAuthLogin saves the given token as the login token
func ControlPanelAuthLogin(token string) {
	rVPNState, err := GetRVpnState()
	if err != nil {
		fmt.Printf("failed to get rVPN state: %v\n", err)
		os.Exit(1)
	}

	rVPNState.ControlPlaneAuth = token
	err = SetRVpnState(rVPNState)
	if err != nil {
		fmt.Println("failed to save rVPN state")
		os.Exit(1)
	}

	fmt.Println("successfully set rVPN login token!")
}

// ClientConnectProfile instructs the rVPN daemon to connect to a target via rpc
func ClientConnectProfile(profile string) {
	// connect to rVPN daemon
	client, err := rpc.Dial("tcp", "127.0.0.1:52370")
	if err != nil {
		fmt.Println("failed to connect to rVPN daemon", err)
		os.Exit(1)
	}

	// ensure device is not already connected
	var connectionStatus RVPNStatus
	err = client.Call("RVPNDaemon.Status", "", &connectionStatus)
	if err != nil {
		fmt.Println("failed to connect rVPN target", err)
		os.Exit(1)
	}

	if connectionStatus == StatusConnected {
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

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("failed to read device registration response", err)
		os.Exit(1)
	}

	deviceRegistrationResp := registerDeviceResponse{}
	err = json.Unmarshal(body, &deviceRegistrationResp)
	if err != nil {
		fmt.Println("failed to unmarshal device registration response", err)
		os.Exit(1)
	}

	// start connection by issuing request to rVPN daemon
	connectionRequest := ConnectRequest{
		Profile:     profile,
		DeviceToken: deviceRegistrationResp.DeviceToken,
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
		fmt.Println("failed to connect to rVPN daemon")
		os.Exit(1)
	}

	// ensure device is connected
	var connectionStatus RVPNStatus
	err = client.Call("RVPNDaemon.Status", "", &connectionStatus)
	if err != nil {
		fmt.Println("failed to connect rVPN target", err)
		os.Exit(1)
	}

	if connectionStatus == StatusDisconnected {
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

	var rVPNState RVPNStatus
	err = client.Call("RVPNDaemon.Status", "", &rVPNState)
	if err != nil {
		fmt.Println("failed to get status from rVPN daemon", err)
		os.Exit(1)
	}

	fmt.Println(rVPNState)
}
