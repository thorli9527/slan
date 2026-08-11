package model

type DeviceCredential struct {
	CredentialID       string `json:"credentialId"`
	KeyID              string `json:"keyId"`
	DeviceID           string `json:"deviceId"`
	Name               string `json:"name"`
	SecretHash         string `json:"-"`
	Status             string `json:"status"`
	Scopes             string `json:"scopes"`
	LastUsedAt         int64  `json:"lastUsedAt"`
	LastUsedIP         string `json:"lastUsedIp"`
	OfflineAckAt       int64  `json:"offlineAckAt"`
	DisableNotifiedAt  int64  `json:"disableNotifiedAt"`
	DisableNotifyCount int64  `json:"disableNotifyCount"`
	CreatedAt          int64  `json:"createdAt"`
	UpdatedAt          int64  `json:"updatedAt"`
	RevokedAt          int64  `json:"revokedAt"`
}

const (
	DeviceCredentialStatusActive  = "active"
	DeviceCredentialStatusRevoked = "revoked"
)
