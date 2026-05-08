package service

import "github.com/slan/server/server-biz/api/dto"

// Auth is the account access surface exposed by server-biz.
//
// Responsibilities:
// - Register/Login/Refresh: issue short-lived access tokens + refresh tokens for end users.
// - Password management: change the current user's password.
// - Browser callback workflow: support "login in browser then deliver session back to device".
// - Web console workflow: issue and consume one-time login keys.
//
// Authentication model:
//   - Register/Login/Refresh/CallbackStatus/CompleteCallback/ConsumeConsoleLoginKey are typically called
//     before a device exists and therefore must be accessible without HTTP Bearer auth.
//   - ChangePassword and CreateConsoleLoginKey are called after login and require an authenticated userID
//     provided by the HTTP layer.
//
// Errors:
//   - Implementations should use service-level errors (ErrUnauthorized/ErrForbidden/ErrConflict/ErrInvalidArgument)
//     so the HTTP layer can map them to stable error codes.
type Auth interface {
	// Register creates a new end-user account and returns a fresh AuthResponse.
	//
	// Input:
	// - req.Email, req.Password
	//
	// Output:
	// - AuthResponse(accessToken, refreshToken, expiresIn, userId, email...)
	//
	// Side effects:
	// - Persists a new user record in the database.
	// - Persists issued tokens in the token store.
	//
	// Typical errors:
	// - ErrForbidden: registration disabled by config.
	// - ErrConflict: email already exists.
	// - ErrInvalidArgument: missing/invalid fields.
	Register(req dto.RegisterRequest) (dto.AuthResponse, error)

	// Login verifies user credentials and returns a fresh AuthResponse.
	//
	// Input:
	// - req.Email, req.Password
	// - req.DeviceID is optional; if present the implementation should validate device ownership.
	//
	// Output:
	// - AuthResponse(accessToken, refreshToken, expiresIn, userId, email...)
	//
	// Typical errors:
	// - ErrUnauthorized: email/password mismatch, unknown email, or invalid device binding.
	// - ErrForbidden: deviceId exists but is owned by another user.
	// - ErrInvalidArgument: missing/invalid fields.
	Login(req dto.LoginRequest) (dto.AuthResponse, error)

	// Refresh consumes a refresh token and returns a new AuthResponse.
	//
	// Notes:
	// - Implementations should treat refresh tokens as single-use (consume + delete) to reduce replay risk.
	// - req.DeviceID is optional; when present it should be validated against the user derived from refreshToken.
	//
	// Typical errors:
	// - ErrUnauthorized: refresh token expired/invalid or device binding invalid.
	// - ErrForbidden: deviceId exists but is owned by another user.
	// - ErrInvalidArgument: missing refreshToken, invalid deviceId.
	Refresh(req dto.RefreshTokenRequest) (dto.AuthResponse, error)

	// ChangePassword updates the current user's password.
	//
	// Input:
	// - userID: current authenticated user ID from the HTTP layer.
	// - req.CurrentPassword, req.NewPassword
	//
	// Typical errors:
	// - ErrUnauthorized: user not found or current password mismatch.
	// - ErrInvalidArgument: missing fields or newPassword does not satisfy constraints.
	ChangePassword(userID string, req dto.ChangePasswordRequest) error

	// CreateConsoleLoginKey issues a short-lived, one-time login key for the web console.
	//
	// Input:
	// - userID: current authenticated user ID from the HTTP layer.
	// - req.DeviceID optional: binds the console login to a specific device context.
	//
	// Output:
	// - ConsoleLoginKeyResponse(loginKey, expiresInSeconds)
	//
	// Typical errors:
	// - ErrUnauthorized: userID invalid.
	// - ErrForbidden: deviceId exists but is owned by another user.
	// - ErrInvalidArgument: invalid deviceId.
	CreateConsoleLoginKey(userID string, req dto.CreateConsoleLoginKeyRequest) (dto.ConsoleLoginKeyResponse, error)

	// ConsumeConsoleLoginKey exchanges a one-time console login key for AuthResponse.
	//
	// Notes:
	// - The key should be consumed atomically to prevent reuse.
	//
	// Typical errors:
	// - ErrUnauthorized: key missing/expired/already consumed.
	// - ErrInvalidArgument: missing loginKey.
	ConsumeConsoleLoginKey(req dto.ConsumeConsoleLoginKeyRequest) (dto.AuthResponse, error)

	// GetCallbackStatus reads the current browser-login callback status for the given callbackID.
	//
	// Use case:
	// - A desktop client polls this endpoint until Ready=true, then applies the returned payload locally.
	//
	// Typical errors:
	// - ErrInvalidArgument: callbackID empty.
	GetCallbackStatus(callbackID string) (dto.AuthCallbackStatusResponse, error)

	// CompleteCallback stores the browser-login callback payload under callbackID.
	//
	// Security:
	// - The payload should be self-authenticated: req.AccessToken must be valid and match req.UserID.
	// - If req.DeviceID is provided, it must belong to req.UserID to avoid cross-device injection.
	//
	// Side effects:
	// - Persists the callback payload with a short TTL for polling.
	// - Optionally publishes the payload to the device via control/down MQTT when available.
	//
	// Typical errors:
	// - ErrUnauthorized: accessToken invalid.
	// - ErrForbidden: user/device mismatch.
	// - ErrInvalidArgument: missing required fields.
	CompleteCallback(callbackID string, req dto.CompleteAuthCallbackRequest) error
}

// TokenVerifier validates HTTP Bearer access tokens.
//
// The HTTP middleware uses it to map an access token to a userID and to reject invalid/expired tokens.
type TokenVerifier interface {
	// Authenticate returns the authenticated userID for the given access token.
	//
	// Typical errors:
	// - ErrUnauthorized: token invalid/expired/revoked.
	Authenticate(accessToken string) (string, error)
}
