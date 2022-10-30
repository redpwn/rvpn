package wg

type WgConfig struct {
	PrivateKey string
	PublicKey  string
	ClientIp   string
	ClientCidr string
	ServerIp   string
	ServerPort int
	DnsIp      string
}
