package service

type DeviceSessionView struct {
	SessionID    string `json:"sessionId"`
	DeviceID     string `json:"deviceId"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	Status       string `json:"status"`
	ExpiresAt    int64  `json:"expiresAt"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

type DeviceSessionBoundView struct {
	Profile DeviceProfileView     `json:"profile"`
	Session DeviceSessionView     `json:"session"`
	MQTT    DeviceMQTTProfileView `json:"mqtt"`
}
