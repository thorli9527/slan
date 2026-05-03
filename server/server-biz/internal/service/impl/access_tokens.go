package impl

import (
	"context"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

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
