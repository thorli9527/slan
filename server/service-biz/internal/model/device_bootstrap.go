package model

type DeviceBootstrapKey struct {
	KeyID          string `json:"keyId"`
	UserID         string `json:"userId"`
	NetworkID      string `json:"networkId"`
	Name           string `json:"name"`
	Token          string `json:"token"`
	Status         string `json:"status"`
	ExpiresAt      int64  `json:"expiresAt"`
	UsedAt         int64  `json:"usedAt"`
	UsedByDeviceID string `json:"usedByDeviceId"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
}
