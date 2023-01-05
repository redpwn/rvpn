package wg

import "golang.zx2c4.com/wireguard/wgctrl/wgtypes"

const (
	IpSourceRouteTableBaseIdx = 130
)

type ClientWgConfig struct {
	ClientPrivateKey string // client private key
	ServerPublicKey  string // server public key
	ClientIp         string
	ClientCidr       string
	ServerIp         string
	ServerPort       int
	DnsIp            string
}

type WireGuardPeer struct {
	PublicKey   string
	AllowedIP   string
	AllowedCidr string
	// TODO: consider adding device id or some identify for indexing to remove peers down the line
}

type ServeWgConfig struct {
	PrivateKey   string // client private key
	ListenPort   int
	InternalIp   string
	InternalCidr string
	Peers        []WireGuardPeer
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
