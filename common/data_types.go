package common

// RegisterDeviceResponse holds the response format for device registration
type RegisterDeviceResponse struct {
	// device id which has been assigned to the device
	DeviceId string `json:"deviceId,omitempty"`

	// device token which is signed and authenticates the device
	DeviceToken string `json:"deviceToken,omitempty"`
}

// ConnectionMetadata holds the format for connection metadata
type ConnectionMetadata struct {
	Subnets []string `json:"subnets"` // i.e ["192.168.5.1/24", "10.10.10.1/24"]
}
