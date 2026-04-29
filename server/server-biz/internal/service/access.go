package service

import "github.com/slan/server/server-biz/api/dto"

// Auth defines account registration, login, and browser callback status.
type Auth interface {
	Register(req dto.RegisterRequest) (dto.AuthResponse, error)
	Login(req dto.LoginRequest) (dto.AuthResponse, error)
	Refresh(req dto.RefreshTokenRequest) (dto.AuthResponse, error)
	ChangePassword(userID string, req dto.ChangePasswordRequest) error
	CreateConsoleLoginKey(userID string, req dto.CreateConsoleLoginKeyRequest) (dto.ConsoleLoginKeyResponse, error)
	ConsumeConsoleLoginKey(req dto.ConsumeConsoleLoginKeyRequest) (dto.AuthResponse, error)
	GetCallbackStatus(callbackID string) (dto.AuthCallbackStatusResponse, error)
	CompleteCallback(callbackID string, req dto.CompleteAuthCallbackRequest) error
}

// TokenVerifier validates access tokens used by the control plane.
type TokenVerifier interface {
	Authenticate(accessToken string) (string, error)
}
