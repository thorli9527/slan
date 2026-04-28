package impl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/service"
	"github.com/slan/server/server-biz/internal/util"
)

type dbAuthService struct{ state *dbState }
type dbTokenVerifier struct{ state *dbState }

var (
	_ service.Auth          = dbAuthService{}
	_ service.TokenVerifier = dbTokenVerifier{}
)

func (s dbAuthService) Register(req dto.RegisterRequest) (dto.AuthResponse, error) {
	if !s.state.cfg.Auth.AllowRegistration {
		return dto.AuthResponse{}, ErrForbidden
	}
	email := strings.TrimSpace(strings.ToLower(req.Email))
	if email == "" || len(req.Password) < 8 {
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
		PasswordHash: util.HashPassword(req.Password),
	}); err != nil {
		return dto.AuthResponse{}, err
	}
	return s.state.issueAuthResponse(ctx, userID, "")
}

func (s dbAuthService) Login(req dto.LoginRequest) (dto.AuthResponse, error) {
	email := strings.TrimSpace(strings.ToLower(req.Email))
	deviceID := strings.TrimSpace(req.DeviceID)
	if email == "" || req.Password == "" {
		return dto.AuthResponse{}, fmt.Errorf("%w: email and password are required", ErrInvalidArgument)
	}
	ctx := context.Background()
	user, err := s.state.pg.GetUserByEmail(ctx, email)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.AuthResponse{}, ErrUnauthorized
		}
		return dto.AuthResponse{}, err
	}
	if user.PasswordHash != util.HashPassword(req.Password) {
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
	if strings.TrimSpace(req.CurrentPassword) == "" || len(req.NewPassword) < 8 {
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
	if user.PasswordHash != util.HashPassword(req.CurrentPassword) {
		return ErrUnauthorized
	}
	return s.state.pg.UpdateUserPassword(ctx, userID, util.HashPassword(req.NewPassword))
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
	if req.ExpiresIn <= 0 {
		req.ExpiresIn = 3600
	}
	return s.state.tokens.StoreAuthCallbackPayload(
		context.Background(),
		callbackID,
		req,
		10*time.Minute,
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
