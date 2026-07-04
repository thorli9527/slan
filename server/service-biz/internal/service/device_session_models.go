package service

type DeviceSessionView struct {
	SessionID    string `json:"sessionId"`
	DeviceID     string `json:"deviceId"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	Status       string `json:"status"`
	SessionMode  string `json:"sessionMode"`
	ExpiresAt    int64  `json:"expiresAt"`
	RefreshExpiry int64 `json:"refreshExpiry"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
	RevokedAt    int64  `json:"revokedAt"`
}

type DeviceSessionBoundView struct {
	Profile DeviceProfileView     `json:"profile"`
	Session DeviceSessionView     `json:"session"`
	MQTT    DeviceMQTTProfileView `json:"mqtt"`
}
