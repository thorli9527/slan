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

type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

type AuthResponse struct {
	UserID       string `json:"userId"`
	Email        string `json:"email,omitempty"`
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
	Payload    *CompleteAuthCallbackRequest `json:"payload,omitempty"`
}

type PublicSystemConfig struct {
	AllowRegistration bool `json:"allowRegistration"`
}
