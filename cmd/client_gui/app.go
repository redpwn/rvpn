package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/rpc"
	"os"
	"path"
	"path/filepath"

	"github.com/denisbrodbeck/machineid"
	"github.com/redpwn/rvpn/common"
	"github.com/redpwn/rvpn/daemon"
	"github.com/redpwn/rvpn/service"
)

const (
	RVPN_CONTROL_PLANE    = "http://rvpn.jimmyli.us"
	RVPN_CONTROL_PLANE_WS = "ws://rvpn.jimmyli.us"
	RVPN_VERSION          = "0.0.1"
)

// App struct
type App struct {
	ctx context.Context
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Launch rVPN daemon service if not debug
	debug := false
	if !debug {
		exePath, err := os.Executable()
		if err != nil {
			return
		}

		exeDirPath := filepath.Dir(exePath)
		cliClientPath := path.Join(exeDirPath, "rvpnc.exe")

		service.StartRVPNDaemon(cliClientPath)
	}
}

// similar to the rVPN cli client, we create bindings to the rVPN daemon

type WrappedReturn struct {
	Success bool   `json:"success"` // bool determining if the request was successful
	Data    string `json:"data"`    // intended data if successful
	Error   string `json:"error"`   // error string if not successful
}

// wrappedError is a helper function which returns an error
func wrappedError(err error) WrappedReturn {
	return WrappedReturn{
		Success: false,
		Data:    "",
		Error:   err.Error(),
	}
}

// wrappedSuccess is a helper function to return data
func wrappedSuccess(data string) WrappedReturn {
	return WrappedReturn{
		Success: true,
		Data:    data,
		Error:   "",
	}
}

// GetRVpnState gets the rVPN state using the rpc client
func GetRVpnState(client *rpc.Client) (daemon.RVpnState, error) {
	// get state from rVPN daemon
	var rVPNState daemon.RVpnState
	err := client.Call("RVPNDaemon.GetState", "", &rVPNState)
	if err != nil {
		return daemon.RVpnState{}, err
	}

	return rVPNState, nil
}

// Set RVpnState sets the rVPN state using the rpc client
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

func (a *App) GetControlPlaneAuth() WrappedReturn {
	// connect to rVPN daemon
	client, err := rpc.Dial("tcp", "127.0.0.1:52370")
	if err != nil {
		return wrappedError(fmt.Errorf("failed to connect to rVPN daemon: %w", err))
	}
	defer client.Close()

	rVPNState, err := GetRVpnState(client)
	if err != nil {
		return wrappedError(fmt.Errorf("failed to get rPVN state: %w", err))
	}

	return wrappedSuccess(rVPNState.ControlPlaneAuth)
}

// Login will log the user in with the specified token
func (a *App) Login(token string) WrappedReturn {
	// connect to rVPN daemon
	client, err := rpc.Dial("tcp", "127.0.0.1:52370")
	if err != nil {
		return wrappedError(fmt.Errorf("failed to connect to rVPN daemon: %w", err))
	}
	defer client.Close()

	rVPNState, err := GetRVpnState(client)
	if err != nil {
		return wrappedError(fmt.Errorf("failed to get rVPN state: %w", err))
	}

	rVPNState.ControlPlaneAuth = token
	err = SetRVpnState(client, rVPNState)
	if err != nil {
		return wrappedError(fmt.Errorf("failed to set rVPN state: %w", err))
	}

	return wrappedSuccess("successfully logged into rVPN!")
}

func (a *App) Logout() WrappedReturn {
	// connect to rVPN daemon
	client, err := rpc.Dial("tcp", "127.0.0.1:52370")
	if err != nil {
		return wrappedError(fmt.Errorf("failed to connect to rVPN daemon: %w", err))
	}
	defer client.Close()

	rVPNState, err := GetRVpnState(client)
	if err != nil {
		return wrappedError(fmt.Errorf("failed to get rVPN state: %w", err))
	}

	rVPNState.ControlPlaneAuth = ""
	err = SetRVpnState(client, rVPNState)
	if err != nil {
		return wrappedError(fmt.Errorf("failed to set rVPN state: %w", err))
	}

	return wrappedSuccess("successfully logged out of rVPN!")
}

// ListTargets lists the profiles the current user can access
// NOTE: data is returned as JSON string
func (a *App) ListTargets() WrappedReturn {
	// connect to rVPN daemon
	client, err := rpc.Dial("tcp", "127.0.0.1:52370")
	if err != nil {
		return wrappedError(fmt.Errorf("failed to connect to rVPN daemon: %w", err))
	}
	defer client.Close()

	// get control plane authentication token
	rVPNState, err := GetRVpnState(client)
	if err != nil {
		return wrappedError(fmt.Errorf("failed to get rVPN state: %w", err))
	}

	controlPanelAuthToken := rVPNState.ControlPlaneAuth
	if controlPanelAuthToken == "" {
		return wrappedError(fmt.Errorf(`not logged into rVPN, login first"`))
	}

	controlPlaneURL := RVPN_CONTROL_PLANE + "/api/v1/target/"

	req, err := http.NewRequest("GET", controlPlaneURL, nil)
	if err != nil {
		return wrappedError(fmt.Errorf("list target profiles request failed: %w", err))
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", controlPanelAuthToken))
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return wrappedError(fmt.Errorf("failed to send list target profiles request: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return wrappedError(fmt.Errorf("invalid rVPN login token, please check target / login token and try again"))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return wrappedError(fmt.Errorf("failed to read user targets response: %w", err))
	}

	return wrappedSuccess(string(body))
}

func (a *App) Connect(profile string, opts common.ClientOptions) WrappedReturn {
	// connect to rVPN daemon
	client, err := rpc.Dial("tcp", "127.0.0.1:52370")
	if err != nil {
		return wrappedError(fmt.Errorf("failed to connect to rVPN daemon: %w", err))
	}
	defer client.Close()

	// ensure device is not already connected
	var connectionStatus daemon.RVPNStatus
	err = client.Call("RVPNDaemon.Status", "", &connectionStatus)
	if err != nil {
		return wrappedError(fmt.Errorf("failed to get rVPN status: %w", err))
	}

	if connectionStatus != daemon.StatusDisconnected {
		// device is already connected, early exit
		return wrappedError(fmt.Errorf("device is already connected to a rVPN target, disconnect and try again"))
	}

	// ensure device is registered for target
	rVPNState, err := GetRVpnState(client)
	if err != nil {
		return wrappedError(fmt.Errorf("failed to get rVPN state: %w", err))
	}

	controlPanelAuthToken := rVPNState.ControlPlaneAuth
	if controlPanelAuthToken == "" {
		return wrappedError(fmt.Errorf(`not logged into rVPN, login first"`))
	}

	machineId, err := machineid.ID()
	if err != nil {
		return wrappedError(fmt.Errorf("failed to get machine id: %w", err))
	}

	controlPlaneURL := RVPN_CONTROL_PLANE + "/api/v1/target/" + profile + "/register_device"
	jsonStr := []byte(fmt.Sprintf(`{"hardwareId":"%s"}`, machineId))

	req, err := http.NewRequest("POST", controlPlaneURL, bytes.NewBuffer(jsonStr))
	if err != nil {
		return wrappedError(fmt.Errorf("register device request failed: %w", err))
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", controlPanelAuthToken))
	req.Header.Set("Content-Type", "application/json")

	httpClient := &http.Client{}
	resp, err := httpClient.Do(req)
	if err != nil {
		return wrappedError(fmt.Errorf("failed to send device registration request: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return wrappedError(fmt.Errorf("invalid rVPN login token, please check target / login token and try again"))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return wrappedError(fmt.Errorf("failed to read device registration response: %w", err))
	}

	deviceRegistrationResp := common.RegisterDeviceResponse{}
	err = json.Unmarshal(body, &deviceRegistrationResp)
	if err != nil {
		return wrappedError(fmt.Errorf("failed to unmarshal device registration response: %w", err))
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
		return wrappedError(fmt.Errorf("failed to connect rVPN target: %w", err))
	}

	return wrappedSuccess("successfully connected to rVPN target")
}

// Disconnect disconnects the rVPN client from the current connected rVPN server
func (a *App) Disconnect() WrappedReturn {
	client, err := rpc.Dial("tcp", "127.0.0.1:52370")
	if err != nil {
		return wrappedError(fmt.Errorf("failed to connect to rVPN daemon: %w", err))
	}
	defer client.Close()

	// ensure device is connected
	var connectionStatus daemon.RVPNStatus
	err = client.Call("RVPNDaemon.Status", "", &connectionStatus)
	if err != nil {
		return wrappedError(fmt.Errorf("failed to get rVPN status: %w", err))
	}

	if connectionStatus == daemon.StatusDisconnected {
		// device is already connected, early exit
		return wrappedError(fmt.Errorf("device is not connected to a rVPN target, disconnect and try again"))
	}

	var disconnectSuccess bool
	err = client.Call("RVPNDaemon.Disconnect", "", &disconnectSuccess)
	if err != nil {
		return wrappedError(fmt.Errorf("failed to disconnect from rVPN connection: %w", err))
	}

	return wrappedSuccess("successfully disconnected from rVPN")
}

// Status gets the current status of the rVPN daemon
func (a *App) Status() WrappedReturn {
	client, err := rpc.Dial("tcp", "127.0.0.1:52370")
	if err != nil {
		return wrappedError(fmt.Errorf("failed to connect to rVPN daemon: %w", err))
	}
	defer client.Close()

	var rVPNState daemon.RVPNStatus
	err = client.Call("RVPNDaemon.Status", "", &rVPNState)
	if err != nil {
		return wrappedError(fmt.Errorf("failed to get status from rVPN daemon: %w", err))
	}

	if rVPNState == daemon.StatusConnected {
		return wrappedSuccess("rVPN is currently connected to a profile")
	} else if rVPNState == daemon.StatusDisconnected {
		return wrappedSuccess("rVPN is not currently connected to a profile")
	} else if rVPNState == daemon.StatusServing {
		return wrappedSuccess("rVPN is currently serving as a target VPN server")
	} else {
		return wrappedSuccess("something went wrong, rVPN status is unrecognized")
	}
}

// Version gets the rVPN version of the client
func (a *App) Version() string {
	return RVPN_VERSION
}
