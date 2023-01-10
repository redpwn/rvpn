package common

import (
	"bufio"
	"encoding/json"
	"os"
	"path"

	"github.com/kirsle/configdir"
)

type RVpnState struct {
	ControlPlaneAuth string `json:"controlplaneauth"` // token which is used to authenticate to the control plane
	PrivateKey       string `json:"privatekey"`
	PublicKey        string `json:"publickey"`
	ActiveProfile    string `json:"activeprofile"` // TODO: remove because deprecated, this logic is moved to the rVPN daemon
}

// getRVpnStatePath gets the rVPN state path from the system
func getRVpnStatePath() (string, error) {
	configPaths := configdir.SystemConfig("rvpn")

	return path.Join(configPaths[0], "state.json"), nil
}

// GetRVpnState returns the parsed rVPN state from the system
func GetRVpnState() (RVpnState, error) {
	rVpnStateFile, err := getRVpnStatePath()
	if err != nil {
		return RVpnState{}, err
	}

	rVpnStateData, err := os.ReadFile(rVpnStateFile)
	if err != nil {
		return RVpnState{}, err
	}

	var rVpnStateObj RVpnState
	json.Unmarshal(rVpnStateData, &rVpnStateObj)
	return rVpnStateObj, nil
}

// SetRVpnState sets the rVPN state on the system
func SetRVpnState(rVpnStateData RVpnState) error {
	rVpnStateFile, err := getRVpnStatePath()
	if err != nil {
		return err
	}

	rVpnStateJson, err := json.Marshal(rVpnStateData)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(rVpnStateFile, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	w := bufio.NewWriter(f)
	_, err = w.Write(rVpnStateJson)
	if err != nil {
		return err
	}

	w.Flush()
	return nil
}

// InitRVPNState initializes the WireGuard client and must be run before any operations
func InitRVPNState() error {
	// create rVPN configuration directory if not exists
	rvpnConfigDir := configdir.SystemConfig("rvpn")[0]
	if _, err := os.Stat(rvpnConfigDir); os.IsNotExist(err) {
		err = os.MkdirAll(rvpnConfigDir, 0600)
		if err != nil {
			return err
		}
	}

	// initialize rVPN state
	rVpnStateFile, err := getRVpnStatePath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(rVpnStateFile); os.IsNotExist(err) {
		// if no rVpnState config then set it to be empty
		SetRVpnState(RVpnState{})
	}

	return nil
}
