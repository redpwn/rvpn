package wg

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"text/template"

	"github.com/redpwn/rvpn/cmd/client/elevate"
)

type RVpnState struct {
	PrivateKey       string `json:"privatekey"`
	PublicKey        string `json:"publickey"`
	ActiveProfile    string `json:"activeprofile"`
	ControlPlaneAuth string `json:"controlplaneauth"`
}

type WgConfig struct {
	PrivateKey string
	PublicKey  string
	ClientIp   string
	ClientCidr string
	ServerIp   string
	ServerPort string
	DnsIp      string
}

//go:embed templates/template.conf
var wgTemplateStr string
var wgTemplate, wgTemplateErr = template.New("wgTemplate").Parse(wgTemplateStr)

func getRVpnStatePath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return path.Join(configDir, "rvpn", "state.json"), nil
}

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

func getWgKeys() (string, string, error) {
	// returns private key, public key, error
	rVpnStateLocal, err := GetRVpnState()
	if err != nil {
		return "", "", nil
	}

	if rVpnStateLocal.PrivateKey == "" {
		// there is no private key, regenerate and set
		privKeyRaw, err := exec.Command("wg", "genkey").Output()
		if err != nil {
			return "", "", err
		}
		privKey := strings.TrimRight(string(privKeyRaw), "\r\n")

		var pubKeyBuf bytes.Buffer
		pubKeyWriter := bufio.NewWriter(&pubKeyBuf)

		cmd := exec.Command("wg", "pubkey")
		cmd.Stdin = strings.NewReader(privKey)
		cmd.Stdout = pubKeyWriter
		err = cmd.Run()
		if err != nil {
			return "", "", err
		}

		pubKey := strings.TrimRight(pubKeyBuf.String(), "\r\n")
		rVpnStateLocal.PrivateKey = privKey
		rVpnStateLocal.PublicKey = pubKey
		SetRVpnState(rVpnStateLocal)
		return privKey, pubKey, nil
	} else {
		return rVpnStateLocal.PrivateKey, rVpnStateLocal.PublicKey, nil
	}
}

func getProfilePath(profile string) (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}

	return path.Join(configDir, "rvpn", "profiles", profile+".conf"), nil
}

func writeWgConfig(profilePath string, userConfig WgConfig) error {
	if wgTemplateErr != nil {
		return wgTemplateErr
	}

	f, err := os.OpenFile(profilePath, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	err = wgTemplate.Execute(w, userConfig)
	if err != nil {
		return err
	}

	w.Flush()

	return nil
}

func ConnectProfile(profile string) error {
	rVpnStateLocal, err := GetRVpnState()
	if err != nil {
		return err
	}

	if rVpnStateLocal.ActiveProfile != "" {
		fmt.Println("already connected to a profile")
		os.Exit(1)
	}

	fmt.Println("connecting to " + profile)

	privKey, pubKey, err := getWgKeys()
	if err != nil {
		return err
	}

	fmt.Println("curr keys: " + privKey + " " + pubKey)

	// TODO: get connection info from controlplane from API
	userConfig := WgConfig{
		PrivateKey: "--",
		PublicKey:  "Xb5+rEyb4eozBWYruk5iA7shr8miaQMka937dagG20c=",
		ClientIp:   "10.8.0.2",
		ClientCidr: "/24",
		ServerIp:   "jmy.li",
		ServerPort: "21820",
		DnsIp:      "1.1.1.1",
	}

	profilePath, err := getProfilePath(profile)
	if err != nil {
		return err
	}

	err = writeWgConfig(profilePath, userConfig)
	if err != nil {
		fmt.Println(err)
	}

	elevate.RunWGCmdElevated("/installtunnelservice " + profilePath)

	// TODO: Do health checks to see if the connection works
	// Potential idea is to ping gateway which we will hardcode to a certain IP or receive from control-plane

	rVpnStateLocal.ActiveProfile = profile
	SetRVpnState(rVpnStateLocal)

	fmt.Println("connected to " + profile)

	return nil
}

func DisconnectProfile() error {
	rVpnStateLocal, err := GetRVpnState()
	if err != nil {
		return err
	}

	if rVpnStateLocal.ActiveProfile == "" {
		fmt.Println("not currently connected to a profile")
		return nil
	}

	elevate.RunWGCmdElevated("/uninstalltunnelservice " + rVpnStateLocal.ActiveProfile)

	rVpnStateLocal.ActiveProfile = ""
	SetRVpnState(rVpnStateLocal)

	return nil
}

func InitWgClient() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	rvpnConfigDir := path.Join(configDir, "rvpn")
	if _, err := os.Stat(rvpnConfigDir); os.IsNotExist(err) {
		err = os.MkdirAll(rvpnConfigDir, 0600)
		if err != nil {
			return err
		}
	}

	rVpnStateFile, err := getRVpnStatePath()
	if err != nil {
		return err
	}

	if _, err := os.Stat(rVpnStateFile); os.IsNotExist(err) {
		// if no rVpnState config then set it to be empty
		SetRVpnState(RVpnState{})
	}

	rVpnProfileDir := path.Join(configDir, "rvpn", "profiles")
	if _, err := os.Stat(rVpnProfileDir); os.IsNotExist(err) {
		err = os.MkdirAll(rVpnProfileDir, 0600)
		if err != nil {
			return err
		}
	}

	return nil
}
