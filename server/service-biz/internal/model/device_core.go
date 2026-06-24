package model

type Device struct {
	DeviceID      string `json:"deviceId"`
	OwnerID       string `json:"ownerId"`
	Name          string `json:"name"`
	Platform      string `json:"platform"`
	Alias         string `json:"alias,omitempty"`
	OSName        string `json:"osName,omitempty"`
	OSVersion     string `json:"osVersion,omitempty"`
	PublicKey     string `json:"publicKey,omitempty"`
	DeviceVersion string `json:"deviceVersion,omitempty"`
	CountryCode   string `json:"countryCode,omitempty"`
	RXBytesTotal  int64  `json:"rxBytesTotal"`
	TXBytesTotal  int64  `json:"txBytesTotal"`
	Status        string `json:"status"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
	LastSeenAt    int64  `json:"lastSeenAt,omitempty"`
}
