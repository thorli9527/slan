package dto

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	DeviceID string `json:"deviceId,omitempty"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken"`
	DeviceID     string `json:"deviceId,omitempty"`
}

type AuthResponse struct {
	UserID       string `json:"userId"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresIn    int64  `json:"expiresIn"`
}

type CompleteAuthCallbackRequest struct {
	AccessToken  string `json:"accessToken"`
	UserID       string `json:"userId"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresIn    int64  `json:"expiresIn"`
	DeviceID     string `json:"deviceId,omitempty"`
	UserLabel    string `json:"userLabel,omitempty"`
	Action       string `json:"action,omitempty"`
}

type AuthCallbackStatusResponse struct {
	CallbackID string                       `json:"callbackId"`
	Ready      bool                         `json:"ready"`
	Received   bool                         `json:"received"`
	ReceivedAt int64                        `json:"receivedAt,omitempty"`
	Payload    *CompleteAuthCallbackRequest `json:"payload,omitempty"`
}
