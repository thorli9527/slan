package model

type UserSession struct {
	SessionID                  string `json:"sessionId"`
	UserID                     string `json:"userId"`
	AccessToken                string `json:"accessToken"`
	RefreshToken               string `json:"refreshToken"`
	Status                     string `json:"status"`
	SessionMode                string `json:"sessionMode"`
	ClientType                 string `json:"clientType"`
	DeviceID                   string `json:"deviceId"`
	PreviousRefreshTokenHash   string `json:"-"`
	RefreshRotationGraceExpiry int64  `json:"-"`
	ExpiresAt                  int64  `json:"expiresAt"`
	RefreshExpiry              int64  `json:"refreshExpiry"`
	CreatedAt                  int64  `json:"createdAt"`
	UpdatedAt                  int64  `json:"updatedAt"`
	RevokedAt                  int64  `json:"revokedAt"`
}
