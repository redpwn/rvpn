//go:build linux

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
)

// ClientServeProfile instructs the rVPN daemon to serve as a VPN server for a target via rpc
func ClientServeProfile(profile string) {
	client, err := rpc.Dial("tcp", "127.0.0.1:52370")
	if err != nil {
		fmt.Println("failed to connect to rVPN daemon")
		os.Exit(1)
	}

	// ensure device is not already connected
	var connectionStatus RVPNStatus
	err = client.Call("RVPNDaemon.Status", "", &connectionStatus)
	if err != nil {
		fmt.Println("failed to connect rVPN target", err)
		os.Exit(1)
	}

	if connectionStatus != StatusDisconnected {
		// device is already connected, early exit
		fmt.Println("device is already connected to a rVPN target, disconnect and try again")
		os.Exit(1)
	}

	// ensure device is registered for target TODO: this code is repeated, abstract this into a function
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

	deviceRegistrationResp := registerDeviceResponse{}
	err = json.Unmarshal(body, &deviceRegistrationResp)
	if err != nil {
		fmt.Println("failed to unmarshal device registration response", err)
		os.Exit(1)
	}

	// start serving connection by issuing request to rVPN daemon
	serveRequest := ConnectRequest{
		Profile:     profile,
		DeviceToken: deviceRegistrationResp.DeviceToken,
	}

	var connectionSuccess bool
	err = client.Call("RVPNDaemon.Serve", serveRequest, &connectionSuccess)
	if err != nil {
		fmt.Printf("failed to serve rVPN target: %v", err)
		os.Exit(1)
	}

	fmt.Printf("rVPN successfully serving as target VPN server for profile%s\n", profile)
}
