package common

// add strict types for jrpc method names
const (
	// jRPC commands from server to client
	GetDeviceAuthMethod        = "get_device_auth"
	GetClientInformationMethod = "get_client_information"
	GetServeInformationMethod  = "get_serve_information"
	ConnectServerMethod        = "connect_server"
	ServeVPNMethod             = "serve_vpn"
	AppendVPNPeersMethod       = "append_vpn_peers"
	DeleteVPNPeersMethod       = "delete_vpn_peers"

	// jRPC commands from client to server
	DeviceHeartbeatMethod = "device_heartbeat"
)

type WireGuardPeer struct {
	PublicKey   string `json:"publickey"`
	AllowedIP   string `json:"allowedip"`
	AllowedCidr string `json:"allowedcidr"`
}

// GetDeviceAuthRequest holds the arguments for get_device_auth request
type GetDeviceAuthRequest struct{}

// GetDeviceAuthResponse holds the arguments for get_device_auth response
type GetDeviceAuthResponse struct {
	Success     bool   `json:"success"`
	DeviceToken string `json:"devicetoken"`
}

// GetClientInformationRequest holds the arguments for get_client_information request
type GetClientInformationRequest struct{}

// GetClientInformationResponse holds the response for get_client_information request
type GetClientInformationResponse struct {
	Success   bool   `json:"success"`
	PublicKey string `json:"publickey"`
}

// GetServeInformationRequest holds the arguments for get_serve_information request
type GetServeInformationRequest struct{}

// GetServeInformationResponse holds the response for get_serve_information request
type GetServeInformationResponse struct {
	Success       bool   `json:"success"`
	PublicKey     string `json:"publickey"`
	PublicVpnPort string `json:"publicvpnport"`
}

// ConnectServerRequest holds the arguments for connect_server request
type ConnectServerRequest struct {
	ServerPublicKey string `json:"serverpublickey"` // this is the public key the client uses to connect
	ClientPublicKey string `json:"clientpublickey"` // we send this to verify that the rVPN state key is correct / synced
	ClientIp        string `json:"clientip"`
	ClientCidr      string `json:"clientcidr"`
	ServerIp        string `json:"serverip"`
	ServerPort      int    `json:"serverport"`
	DnsIp           string `json:"dnsip"`
}

// ConnectServerResponse holds the response for connect_server request
type ConnectServerResponse struct {
	Success bool `json:"success"`
}

// ServeVPNRequest holds the arguments for the serve_vpn request
type ServeVPNRequest struct {
	ServerPublicKey     string          `json:"serverpublickey"` // we send this to verify that the rVPN state key is correct / synced
	ServerInternalIp    string          `json:"serverinternalip"`
	ServerInternalCidr  string          `json:"serverinternalcidr"`
	ServerPublicVPNPort int             `json:"serverpublicvpnport"`
	Peers               []WireGuardPeer `json:"peers"`
}

// ServeVPNResponse holds the response for the serve_vpn request
type ServeVPNResponse struct {
	Success bool `json:"success"`
}

// AppendVPNPeersRequest holds the arguments for the append_vpn_peers request to append peers
type AppendVPNPeersRequest struct {
	Peers []WireGuardPeer `json:"peers"`
}

// AppendVPNPeersResponse holds the response for the append_vpn_peers request to append peers
type AppendVPNPeersResponse struct {
	Success bool `json:"success"`
}

// DeleteVPNPeersRequest holds the arguments for the delete_vpn_peers request to delete peers
type DeleteVPNPeersRequest struct {
	Peers []WireGuardPeer `json:"peers"`
}

// DeleteVPNPeersResponse holds the response for the delete_vpn_peers request to delete peers
type DeleteVPNPeersResponse struct {
	Success bool `json:"success"`
}

// DeviceHeartbeatRequest holds the arguments for the device_heartbeat request to indicate aliveness of the device
type DeviceHeartbeatRequest struct{}

// DeviceHeartbeatResponse holds the response for the device_heartbeat request to indicate aliveness of the device
type DeviceHeartbeatResponse struct {
	Success bool `json:"success"`
}
