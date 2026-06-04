package biz

import (
	"strings"
	"time"
)

// AuthService 承载认证、会话、控制台登录和设备引导相关业务实现。
type AuthService struct {
	store BusinessStore
}

func (s AuthService) RegisterUser(req RegisterUserRequest) (RegisterUserResponse, error) {
	auth, network, err := s.store.RegisterUser(req.Email, req.Password, req.Name)
	if err != nil {
		return RegisterUserResponse{}, err
	}
	return RegisterUserResponse{Auth: auth, DefaultNetwork: network}, nil
}

func (s AuthService) LoginUser(req LoginUserRequest, remoteIP string) (AuthResponse, error) {
	return s.store.LoginUserWithRateLimit(req.Email, req.Password, remoteIP)
}

func (s AuthService) Logout(accessToken, deviceToken string) (AuthResponse, error) {
	auth, _ := s.store.AuthByToken(accessToken)
	return auth, s.store.LogoutSessions(accessToken, deviceToken)
}

func (s AuthService) RenewUserSession(accessToken string) (AuthResponse, error) {
	return s.store.RenewUserSession(accessToken)
}

func (s AuthService) ChangeUserPassword(userID string, req ChangeUserPasswordRequest) error {
	return s.store.ChangeUserPassword(userID, req.OldPassword, req.NewPassword)
}

func (s AuthService) ListUsers() []User {
	return s.store.ListUsers()
}

func (s AuthService) UserEntitlement(userID string) (DeviceQuota, error) {
	return s.store.DeviceQuota(userID)
}

func (s AuthService) ListUserAliases(ownerUserID string) []UserAlias {
	return s.store.ListUserAliases(ownerUserID)
}

func (s AuthService) UpsertUserAlias(req UpsertUserAliasRequest) (UserAlias, error) {
	return s.store.UpsertUserAlias(req.OwnerUserID, req.Email, req.Alias)
}

func (s AuthService) CreateConsoleLoginKey(accessToken string, req CreateConsoleLoginKeyRequest) (ConsoleLoginKey, error) {
	return s.store.CreateConsoleLoginKey(accessToken, req.DeviceID, 2*time.Minute)
}

func (s AuthService) ConsoleLogin(req ConsoleLoginRequest) (AuthResponse, error) {
	return s.store.ConsumeConsoleLoginKey(req.LoginKey)
}

func (s AuthService) PrepareDeviceLogin(req DeviceIdentityRequest, remoteIP string) (Device, error) {
	if err := s.store.CheckDeviceLoginPrepareRateLimit(req.DeviceID, remoteIP); err != nil {
		return Device{}, err
	}
	return s.store.PrepareDeviceLoginDevice(req.DeviceID, req.Name, req.Platform, req.OSName, req.OSVersion, req.Alias, req.PublicKey)
}

func (s AuthService) CompleteDeviceLogin(deviceID string, req CompleteDeviceLoginRequest) (DeviceUserLoginPayload, error) {
	token := req.AccessToken
	if strings.TrimSpace(token) == "" {
		token = req.Token
	}
	return s.store.CompleteDeviceLoginForDevice(deviceID, token, req.Action)
}

func (s AuthService) CreateDeviceBootstrapKey(accessToken string, req CreateDeviceBootstrapKeyRequest) (DeviceBootstrapKey, error) {
	userID := s.userIDFromRequestOrToken(req.UserID, accessToken)
	return s.store.CreateDeviceBootstrapKey(userID, req.NetworkID, req.DeviceAlias, req.TTLSeconds)
}

func (s AuthService) ListDeviceBootstrapKeys(accessToken, userID string) []DeviceBootstrapKey {
	return s.store.ListDeviceBootstrapKeys(s.userIDFromRequestOrToken(userID, accessToken))
}

func (s AuthService) RevokeDeviceBootstrapKey(accessToken, keyID string, req RevokeDeviceBootstrapKeyRequest) (DeviceBootstrapKey, error) {
	return s.store.RevokeDeviceBootstrapKey(keyID, s.userIDFromRequestOrToken(req.UserID, accessToken))
}

func (s AuthService) userIDFromRequestOrToken(userID, accessToken string) string {
	userID = strings.TrimSpace(userID)
	if userID != "" {
		return userID
	}
	if auth, err := s.store.AuthByToken(accessToken); err == nil {
		return auth.User.UserID
	}
	return ""
}
