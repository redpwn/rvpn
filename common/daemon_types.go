package common

// ClientOptions holds the options for the client
type ClientOptions struct {
	Subnets []string `json:"subnets"` // subnets to connect to which overrides instructions from server
}
