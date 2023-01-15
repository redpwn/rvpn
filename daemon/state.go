package daemon

import (
	"bufio"
	"encoding/json"
	"os"
	"path"
)

type RVpnState struct {
	ControlPlaneAuth string `json:"controlplaneauth"` // token which is used to authenticate to the control plane
	PrivateKey       string `json:"privatekey"`
	PublicKey        string `json:"publickey"`
	ActiveProfile    string `json:"activeprofile"` // TODO: remove because deprecated, this logic is moved to the rVPN daemon
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
	// initialize rVPN state
	rVpnStateFile, err := getRVpnStatePath()
	if err != nil {
		return err
	}

	// create rVPN state directory if not exists
	rVpnConfigDir := path.Dir(rVpnStateFile)
	if _, err := os.Stat(rVpnConfigDir); os.IsNotExist(err) {
		err = os.MkdirAll(rVpnConfigDir, 0600)
		if err != nil {
			return err
		}
	}

	if _, err := os.Stat(rVpnStateFile); os.IsNotExist(err) {
		// if no rVpnState config then set it to be empty
		SetRVpnState(RVpnState{})
	}

	return nil
}
