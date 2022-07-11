// Package main provides primitives to interact with the openapi HTTP API.
//
// Code generated by github.com/deepmap/oapi-codegen version v1.11.0 DO NOT EDIT.
package main

const (
	BearerAuthScopes = "bearerAuth.Scopes"
)

// ConnectionConfig defines model for ConnectionConfig.
type ConnectionConfig struct {
	// cidr for the client ip (e.g /24)
	ClientCidr *string `json:"clientCidr,omitempty"`

	// ip for the client to have on the VPN network
	ClientIp *string `json:"clientIp,omitempty"`

	// ip of the DNS server for the client to use
	DnsIp *string `json:"dnsIp,omitempty"`

	// public key to use for wg profile
	PublicKey *string `json:"publicKey,omitempty"`

	// ip for the rVPN server
	ServerIp *string `json:"serverIp,omitempty"`

	// port for the rVPN server
	ServerPort *string `json:"serverPort,omitempty"`
}

// Error defines model for Error.
type Error struct {
	Error struct {
		// Human-readable error description
		Message string `json:"message"`
	} `json:"error"`
}

// GetConnectionResponse defines model for GetConnectionResponse.
type GetConnectionResponse struct {
	Config ConnectionConfig `json:"config"`
}

// ListTargetsResponse defines model for ListTargetsResponse.
type ListTargetsResponse = []struct {
	Name string `json:"name"`
}

// NewConnectionRequest defines model for NewConnectionRequest.
type NewConnectionRequest struct {
	// human-readable machine name
	Name string `json:"name"`

	// public key associated with the device
	Pubkey string `json:"pubkey"`
}

// NewConnectionResponse defines model for NewConnectionResponse.
type NewConnectionResponse struct {
	Config *ConnectionConfig `json:"config,omitempty"`
	Id     *string           `json:"id,omitempty"`
}

// UpdateTarget defines model for UpdateTarget.
type UpdateTarget struct {
	// action to complete for user (modify / delete)
	Action *string `json:"action,omitempty"`

	// email of the user to modify
	UserEmail *string `json:"userEmail,omitempty"`

	// type of user (admin / user), if modifying
	UserType *string `json:"userType,omitempty"`
}

// Id defines model for id.
type Id = string

// Target defines model for target.
type Target = string

// Unauthorized defines model for Unauthorized.
type Unauthorized = Error

// GetAuthLoginParams defines parameters for GetAuthLogin.
type GetAuthLoginParams struct {
	// code response from Google OAuth
	Code string `form:"code" json:"code"`
}

// PatchTargetTargetJSONBody defines parameters for PatchTargetTarget.
type PatchTargetTargetJSONBody = UpdateTarget

// PostTargetTargetConnectionJSONBody defines parameters for PostTargetTargetConnection.
type PostTargetTargetConnectionJSONBody = NewConnectionRequest

// PatchTargetTargetJSONRequestBody defines body for PatchTargetTarget for application/json ContentType.
type PatchTargetTargetJSONRequestBody = PatchTargetTargetJSONBody

// PostTargetTargetConnectionJSONRequestBody defines body for PostTargetTargetConnection for application/json ContentType.
type PostTargetTargetConnectionJSONRequestBody = PostTargetTargetConnectionJSONBody