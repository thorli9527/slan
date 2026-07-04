package service

type CreateDeviceBootstrapKeyInput struct {
	UserID      string `json:"userId"`
	ActorUserID string `json:"actorUserId"`
	NetworkID   string `json:"networkId"`
	Name        string `json:"name"`
	TTLSeconds  int64  `json:"ttlSeconds"`
	ExpiresAt   int64  `json:"expiresAt"`
}

type RevokeDeviceBootstrapKeyInput struct {
	KeyID       string `json:"keyId"`
	ActorUserID string `json:"actorUserId"`
}

type BootstrapDeviceSessionInput struct {
	InstallationKey string `json:"installationKey"`
	SessionKey      string `json:"sessionKey"`
	SessionMode     string `json:"sessionMode"`
	DeviceID      string `json:"deviceId"`
	OwnerID       string `json:"ownerId"`
	Name          string `json:"name"`
	Platform      string `json:"platform"`
	Alias         string `json:"alias"`
	OSName        string `json:"osName"`
	OSVersion     string `json:"osVersion"`
	PublicKey     string `json:"publicKey"`
	DeviceVersion string `json:"deviceVersion"`
	CountryCode   string `json:"countryCode"`
}
