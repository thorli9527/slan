package model

type DeviceLoginDevice struct {
	DeviceID      string `json:"deviceId"`
	UserID        string `json:"userId"`
	Name          string `json:"name"`
	Platform      string `json:"platform"`
	Alias         string `json:"alias"`
	OSName        string `json:"osName"`
	OSVersion     string `json:"osVersion"`
	PublicKey     string `json:"publicKey"`
	DeviceVersion string `json:"deviceVersion"`
	CountryCode   string `json:"countryCode"`
	VerifyCode    string `json:"verifyCode"`
	Status        string `json:"status"`
	ExpiresAt     int64  `json:"expiresAt"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
}
