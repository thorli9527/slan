package service

type CreateDeviceCredentialInput struct {
	DeviceID string `json:"deviceId"`
	Name     string `json:"name"`
	Scopes   string `json:"scopes"`
}

type ExchangeDeviceCredentialInput struct {
	Key           string `json:"key"`
	DeviceID      string `json:"deviceId"`
	Platform      string `json:"platform"`
	DeviceVersion string `json:"deviceVersion"`
	RemoteIP      string `json:"-"`
}

type DeviceCredentialView struct {
	CredentialID string `json:"credentialId"`
	KeyID        string `json:"keyId"`
	DeviceID     string `json:"deviceId"`
	Name         string `json:"name"`
	Scopes       string `json:"scopes"`
	Status       string `json:"status"`
	LastUsedAt   int64  `json:"lastUsedAt"`
	LastUsedIP   string `json:"lastUsedIp"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
	RevokedAt    int64  `json:"revokedAt"`
}

type CreatedDeviceCredentialView struct {
	DeviceCredentialView
	Key string `json:"key"`
}
