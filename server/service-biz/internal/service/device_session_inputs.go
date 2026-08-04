package service

type RenewDeviceSessionInput struct {
	RefreshToken   string `json:"refreshToken"`
	RemoteIP       string `json:"-"`
	NetworkEnabled *bool  `json:"networkEnabled"`
	RXBytesTotal   int64  `json:"rxBytesTotal"`
	TXBytesTotal   int64  `json:"txBytesTotal"`
	LastSeenAt     int64  `json:"lastSeenAt"`
}
