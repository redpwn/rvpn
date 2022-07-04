package wg

import (
	"bufio"
	_ "embed"
	"fmt"
	"os"
	"path"
	"text/template"

	"github.com/redpwn/rvpn/cmd/client/elevate"
)

type WgConfig struct {
	PrivateKey string
	PublicKey  string
	ClientIp   string
	ClientCidr string
	ServerIp   string
	ServerPort string
	DnsIP      string
}

//go:embed templates/template.conf
var wgTemplateStr string
var wgTemplate, wgTemplateErr = template.New("wgTemplate").Parse(wgTemplateStr)

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
	fmt.Println("connecting to " + profile)

	// TODO: get wgconfig from API
	userConfig := WgConfig{
		PrivateKey: "--",
		PublicKey:  "Xb5+rEyb4eozBWYruk5iA7shr8miaQMka937dagG20c=",
		ClientIp:   "10.8.0.2",
		ClientCidr: "/24",
		ServerIp:   "jmy.li",
		ServerPort: "21820",
		DnsIP:      "1.1.1.1",
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

	return nil
}

func InitWgClient() error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	rvpnConfigDir := path.Join(configDir, "rvpn", "profiles")
	if _, err := os.Stat(rvpnConfigDir); os.IsNotExist(err) {
		err = os.MkdirAll(rvpnConfigDir, 0600)
		if err != nil {
			return err
		}
	}

	return nil
}
