package service

type UserManagedSessionView struct {
	SessionID     string `json:"sessionId"`
	UserID        string `json:"userId"`
	Status        string `json:"status"`
	SessionMode   string `json:"sessionMode"`
	ExpiresAt     int64  `json:"expiresAt"`
	RefreshExpiry int64  `json:"refreshExpiry"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
	RevokedAt     int64  `json:"revokedAt"`
}

type DeviceManagedSessionView struct {
	SessionID     string `json:"sessionId"`
	DeviceID      string `json:"deviceId"`
	Status        string `json:"status"`
	SessionMode   string `json:"sessionMode"`
	ExpiresAt     int64  `json:"expiresAt"`
	RefreshExpiry int64  `json:"refreshExpiry"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
	RevokedAt     int64  `json:"revokedAt"`
}
