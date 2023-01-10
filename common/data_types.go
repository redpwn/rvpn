package common

type ConnectionMetadata struct {
	Subnets []string `json:"subnets"` // i.e ["192.168.5.1/24", "10.10.10.1/24"]
}
