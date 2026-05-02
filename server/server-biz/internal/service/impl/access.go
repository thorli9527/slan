package impl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
	"github.com/slan/server/server-biz/internal/mqttauth"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/service"
	"github.com/slan/server/server-biz/internal/util"
)

type dbAuthService struct{ state *dbState }
type dbTokenVerifier struct{ state *dbState }

type consoleLoginKeyPayload struct {
	UserID   string `json:"userId"`
	DeviceID string `json:"deviceId,omitempty"`
}

var (
	_ service.Auth          = dbAuthService{}
	_ service.TokenVerifier = dbTokenVerifier{}
)

func (s dbAuthService) Register(req dto.RegisterRequest) (dto.AuthResponse, error) {
	if !s.state.cfg.Auth.AllowRegistration {
		return dto.AuthResponse{}, ErrForbidden
	}
	email := strings.TrimSpace(strings.ToLower(req.Email))
	password := strings.TrimSpace(req.Password)
	if email == "" || len(password) < 8 {
		return dto.AuthResponse{}, fmt.Errorf("%w: email and password are required", ErrInvalidArgument)
	}

	ctx := context.Background()
	if _, err := s.state.pg.GetUserByEmail(ctx, email); err == nil {
		return dto.AuthResponse{}, fmt.Errorf("%w: email already exists", ErrConflict)
	} else if !repo.IsNotFound(err) {
		return dto.AuthResponse{}, err
	}

	userID := util.NewID("user")
	if err := s.state.pg.CreateUser(ctx, repo.User{
		UserID:       userID,
		Email:        email,
		PasswordHash: util.HashPassword(password),
	}); err != nil {
		return dto.AuthResponse{}, err
	}
	return s.state.issueAuthResponse(ctx, userID, "")
}

func (s dbAuthService) Login(req dto.LoginRequest) (dto.AuthResponse, error) {
	email := strings.TrimSpace(strings.ToLower(req.Email))
	password := strings.TrimSpace(req.Password)
	deviceID := strings.TrimSpace(req.DeviceID)
	if email == "" || password == "" {
		return dto.AuthResponse{}, fmt.Errorf("%w: email and password are required", ErrInvalidArgument)
	}
	if deviceID != "" && !usableClientDeviceID(deviceID) {
		return dto.AuthResponse{}, fmt.Errorf("%w: invalid deviceId", ErrInvalidArgument)
	}
	ctx := context.Background()
	user, err := s.state.pg.GetUserByEmail(ctx, email)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.AuthResponse{}, ErrUnauthorized
		}
		return dto.AuthResponse{}, err
	}
	if user.PasswordHash != util.HashPassword(password) && user.PasswordHash != util.HashPassword(req.Password) {
		return dto.AuthResponse{}, ErrUnauthorized
	}
	if deviceID != "" {
		device, err := s.state.pg.GetDeviceByID(ctx, deviceID)
		if err != nil {
			if repo.IsNotFound(err) {
				return dto.AuthResponse{}, ErrUnauthorized
			}
			return dto.AuthResponse{}, err
		}
		if device.UserID != user.UserID {
			return dto.AuthResponse{}, ErrForbidden
		}
	}
	return s.state.issueAuthResponse(ctx, user.UserID, deviceID)
}

func (s dbAuthService) Refresh(req dto.RefreshTokenRequest) (dto.AuthResponse, error) {
	refreshToken := strings.TrimSpace(req.RefreshToken)
	deviceID := strings.TrimSpace(req.DeviceID)
	if refreshToken == "" {
		return dto.AuthResponse{}, fmt.Errorf("%w: refreshToken is required", ErrInvalidArgument)
	}
	if deviceID != "" && !usableClientDeviceID(deviceID) {
		return dto.AuthResponse{}, fmt.Errorf("%w: invalid deviceId", ErrInvalidArgument)
	}
	ctx := context.Background()
	userID, err := s.state.tokens.AuthenticateRefreshToken(ctx, refreshToken)
	if err != nil {
		return dto.AuthResponse{}, ErrUnauthorized
	}
	if err := s.state.tokens.DeleteRefreshToken(ctx, refreshToken); err != nil {
		return dto.AuthResponse{}, err
	}
	if deviceID != "" {
		device, err := s.state.pg.GetDeviceByID(ctx, deviceID)
		if err != nil {
			if repo.IsNotFound(err) {
				return dto.AuthResponse{}, ErrUnauthorized
			}
			return dto.AuthResponse{}, err
		}
		if device.UserID != userID {
			return dto.AuthResponse{}, ErrForbidden
		}
	}
	return s.state.issueAuthResponse(ctx, userID, deviceID)
}

func (s dbAuthService) ChangePassword(userID string, req dto.ChangePasswordRequest) error {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ErrUnauthorized
	}
	currentPassword := strings.TrimSpace(req.CurrentPassword)
	newPassword := strings.TrimSpace(req.NewPassword)
	if currentPassword == "" || len(newPassword) < 8 {
		return fmt.Errorf("%w: currentPassword and newPassword are required, newPassword must be at least 8 characters", ErrInvalidArgument)
	}
	ctx := context.Background()
	user, err := s.state.pg.GetUserByID(ctx, userID)
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrUnauthorized
		}
		return err
	}
	if user.PasswordHash != util.HashPassword(currentPassword) && user.PasswordHash != util.HashPassword(req.CurrentPassword) {
		return ErrUnauthorized
	}
	return s.state.pg.UpdateUserPassword(ctx, userID, util.HashPassword(newPassword))
}

func (s dbAuthService) CreateConsoleLoginKey(userID string, req dto.CreateConsoleLoginKeyRequest) (dto.ConsoleLoginKeyResponse, error) {
	userID = strings.TrimSpace(userID)
	deviceID := strings.TrimSpace(req.DeviceID)
	if userID == "" {
		return dto.ConsoleLoginKeyResponse{}, ErrUnauthorized
	}
	if deviceID != "" && !usableClientDeviceID(deviceID) {
		return dto.ConsoleLoginKeyResponse{}, fmt.Errorf("%w: invalid deviceId", ErrInvalidArgument)
	}
	ctx := context.Background()
	if _, err := s.state.pg.GetUserByID(ctx, userID); err != nil {
		if repo.IsNotFound(err) {
			return dto.ConsoleLoginKeyResponse{}, ErrUnauthorized
		}
		return dto.ConsoleLoginKeyResponse{}, err
	}
	if deviceID != "" {
		device, err := s.state.pg.GetDeviceByID(ctx, deviceID)
		if err != nil {
			if repo.IsNotFound(err) {
				return dto.ConsoleLoginKeyResponse{}, ErrForbidden
			}
			return dto.ConsoleLoginKeyResponse{}, err
		}
		if device.UserID != userID {
			return dto.ConsoleLoginKeyResponse{}, ErrForbidden
		}
	}
	ttl := 2 * time.Minute
	loginKey := util.OpaqueToken("console", userID)
	if err := s.state.tokens.StoreConsoleLoginKey(ctx, loginKey, consoleLoginKeyPayload{
		UserID:   userID,
		DeviceID: deviceID,
	}, ttl); err != nil {
		return dto.ConsoleLoginKeyResponse{}, err
	}
	return dto.ConsoleLoginKeyResponse{
		LoginKey:  loginKey,
		ExpiresIn: int64(ttl / time.Second),
	}, nil
}

func (s dbAuthService) ConsumeConsoleLoginKey(req dto.ConsumeConsoleLoginKeyRequest) (dto.AuthResponse, error) {
	loginKey := strings.TrimSpace(req.LoginKey)
	if loginKey == "" {
		return dto.AuthResponse{}, ErrInvalidArgument
	}
	ctx := context.Background()
	var payload consoleLoginKeyPayload
	ok, err := s.state.tokens.ConsumeConsoleLoginKey(ctx, loginKey, &payload)
	if err != nil {
		return dto.AuthResponse{}, err
	}
	if !ok || strings.TrimSpace(payload.UserID) == "" {
		return dto.AuthResponse{}, ErrUnauthorized
	}
	return s.state.issueAuthResponse(ctx, payload.UserID, payload.DeviceID)
}

func (s dbAuthService) GetCallbackStatus(callbackID string) (dto.AuthCallbackStatusResponse, error) {
	callbackID = strings.TrimSpace(callbackID)
	if callbackID == "" {
		return dto.AuthCallbackStatusResponse{}, ErrInvalidArgument
	}
	var payload dto.CompleteAuthCallbackRequest
	ready, err := s.state.tokens.LoadAuthCallbackPayload(context.Background(), callbackID, &payload)
	if err != nil {
		return dto.AuthCallbackStatusResponse{}, err
	}
	var responsePayload *dto.CompleteAuthCallbackRequest
	if ready {
		responsePayload = &payload
	}
	return dto.AuthCallbackStatusResponse{
		CallbackID: callbackID,
		Ready:      ready,
		Payload:    responsePayload,
	}, nil
}

func (s dbAuthService) CompleteCallback(callbackID string, req dto.CompleteAuthCallbackRequest) error {
	callbackID = strings.TrimSpace(callbackID)
	if callbackID == "" {
		return ErrInvalidArgument
	}
	req.AccessToken = strings.TrimSpace(req.AccessToken)
	req.UserID = strings.TrimSpace(req.UserID)
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	req.UserLabel = strings.TrimSpace(req.UserLabel)
	req.Action = strings.TrimSpace(req.Action)
	if req.AccessToken == "" || req.UserID == "" {
		return ErrInvalidArgument
	}
	if req.DeviceID != "" && !usableClientDeviceID(req.DeviceID) {
		return fmt.Errorf("%w: invalid deviceId", ErrInvalidArgument)
	}
	ctx := context.Background()
	session, err := s.state.tokens.Authenticate(ctx, req.AccessToken)
	if err != nil {
		return ErrUnauthorized
	}
	if session.UserID != req.UserID {
		return ErrForbidden
	}
	if req.DeviceID != "" {
		device, err := s.state.pg.GetDeviceByID(ctx, req.DeviceID)
		if err != nil {
			if repo.IsNotFound(err) {
				return ErrForbidden
			}
			return err
		}
		if device.UserID != req.UserID {
			return ErrForbidden
		}
	}
	if req.ExpiresIn <= 0 {
		req.ExpiresIn = 3600
	}
	if err := s.state.tokens.StoreAuthCallbackPayload(
		ctx,
		callbackID,
		req,
		10*time.Minute,
	); err != nil {
		return err
	}
	s.state.publishAuthCallbackToDevice(ctx, callbackID, req)
	return nil
}

func (s *dbState) publishAuthCallbackToDevice(ctx context.Context, callbackID string, payload dto.CompleteAuthCallbackRequest) {
	deviceID := strings.TrimSpace(payload.DeviceID)
	if deviceID == "" || !s.cfg.MQTT.Enabled {
		return
	}
	credential := mqttauth.ServerSubscriberCredential(s.cfg.MQTT, time.Now())
	if credential == nil {
		return
	}
	timeout := time.Duration(s.cfg.MQTT.PublishTimeoutMilliseconds) * time.Millisecond
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	publishCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_ = mqttauth.PublishJSONWithOptions(
		publishCtx,
		s.cfg.MQTT,
		credential.ClientID,
		credential.Username,
		credential.Password,
		mqttauth.ControlDownTopic(s.cfg.MQTT, deviceID),
		controlmsg.Envelope{
			Type:      "auth_callback",
			MessageID: util.NewID("msg"),
			Payload: map[string]any{
				"callbackId":   callbackID,
				"accessToken":  payload.AccessToken,
				"refreshToken": payload.RefreshToken,
				"userId":       payload.UserID,
				"userLabel":    payload.UserLabel,
				"deviceId":     payload.DeviceID,
				"expiresIn":    payload.ExpiresIn,
				"action":       payload.Action,
			},
		},
		mqttauth.PublishOptions{QoS: mqttauth.PublishQoSExactlyOnce},
	)
}

func (s *dbState) issueAuthResponse(ctx context.Context, userID, deviceID string) (dto.AuthResponse, error) {
	email := ""
	user, err := s.pg.GetUserByID(ctx, userID)
	if err == nil {
		email = user.Email
	} else if !repo.IsNotFound(err) {
		return dto.AuthResponse{}, err
	}
	accessToken := util.OpaqueToken("access", userID)
	refreshToken := util.OpaqueToken("refresh", userID)
	accessTTL := tokenTTL(s.cfg.Auth.AccessTokenTTLSeconds, time.Hour)
	refreshTTL := tokenTTL(s.cfg.Auth.RefreshTokenTTLSeconds, 24*time.Hour)
	if err := s.tokens.StoreAccessToken(ctx, accessToken, userID, deviceID, accessTTL); err != nil {
		return dto.AuthResponse{}, err
	}
	if err := s.tokens.StoreRefreshToken(ctx, refreshToken, userID, refreshTTL); err != nil {
		return dto.AuthResponse{}, err
	}
	return dto.AuthResponse{
		UserID:       userID,
		Email:        email,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(accessTTL / time.Second),
	}, nil
}

func tokenTTL(seconds int, fallback time.Duration) time.Duration {
	if seconds <= 0 {
		return fallback
	}
	return time.Duration(seconds) * time.Second
}

func (v dbTokenVerifier) Authenticate(accessToken string) (string, error) {
	ctx := context.Background()
	session, err := v.state.tokens.Authenticate(ctx, accessToken)
	if err != nil {
		return "", ErrUnauthorized
	}
	if session.DeviceID != "" && !v.state.hasFreshDeviceBoundWebSession(ctx, session, time.Now()) {
		_ = v.state.tokens.DeleteAccessToken(ctx, accessToken)
		return "", ErrUnauthorized
	}
	return session.UserID, nil
}
