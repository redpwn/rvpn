package common

// add strict types for jrpc method names
const (
	GetClientInformationMethod = "get_client_information"
	ConnectServerMethod        = "connect_server"
)

// GetClientInformationRequest holds the arguments for get_client_information request
type GetClientInformationRequest struct{}

// GetClientInformationResponse holds the response for get_client_information request
type GetClientInformationResponse struct {
	Success   bool   `json:"success"`
	PublicKey string `json:"publickey"`
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
