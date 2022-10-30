package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/redpwn/rvpn/cmd/client/elevate"
)

// getWgKeys gets the wireguard keys from the rVPN state
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

func ConnectProfileOLD(profile string) error {
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

	// TODO: Do health checks to see if the connection works
	// Potential idea is to ping gateway which we will hardcode to a certain IP or receive from control-plane

	rVpnStateLocal.ActiveProfile = profile
	SetRVpnState(rVpnStateLocal)

	fmt.Println("connected to " + profile)

	return nil
}

func DisconnectProfileOLD() error {
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
