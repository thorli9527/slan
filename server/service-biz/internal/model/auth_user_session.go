package model

type UserSession struct {
	SessionID     string `json:"sessionId"`
	UserID        string `json:"userId"`
	AccessToken   string `json:"accessToken"`
	RefreshToken  string `json:"refreshToken"`
	ExpiresAt     int64  `json:"expiresAt"`
	RefreshExpiry int64  `json:"refreshExpiry"`
	CreatedAt     int64  `json:"createdAt"`
}
