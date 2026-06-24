package model

type DeviceEndpoint struct {
	Type      string `json:"type"`
	Address   string `json:"address"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
}
