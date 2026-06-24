package service

type BindDeviceSessionInput struct {
	UserID        string `json:"userId"`
	DeviceID      string `json:"deviceId"`
	Name          string `json:"name"`
	Platform      string `json:"platform"`
	Alias         string `json:"alias"`
	OSName        string `json:"osName"`
	OSVersion     string `json:"osVersion"`
	PublicKey     string `json:"publicKey"`
	DeviceVersion string `json:"deviceVersion"`
	CountryCode   string `json:"countryCode"`
	RXBytesTotal  int64  `json:"rxBytesTotal"`
	TXBytesTotal  int64  `json:"txBytesTotal"`
	LastSeenAt    int64  `json:"lastSeenAt"`
}

type RenewDeviceSessionInput struct {
	NetworkEnabled *bool `json:"networkEnabled"`
	RXBytesTotal   int64 `json:"rxBytesTotal"`
	TXBytesTotal   int64 `json:"txBytesTotal"`
	LastSeenAt     int64 `json:"lastSeenAt"`
}
