package wg

import "golang.zx2c4.com/wireguard/wgctrl/wgtypes"

type WgConfig struct {
	PrivateKey string
	PublicKey  string
	ClientIp   string
	ClientCidr string
	ServerIp   string
	ServerPort int
	DnsIp      string
}

// GenerateKeyPair returns a new private key, public key, and optionally error
func GenerateKeyPair() (string, string, error) {
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return "", "", err
	}

	publicKey := privateKey.PublicKey()

	return privateKey.String(), publicKey.String(), nil
}
