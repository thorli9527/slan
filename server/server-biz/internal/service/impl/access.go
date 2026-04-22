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
	if _, err := s.state.ensureOwnedNetwork(ctx, userID); err != nil {
		return dto.AuthResponse{}, err
	}
	return s.state.issueAuthResponse(ctx, userID)
}

func (s dbAuthService) Login(req dto.LoginRequest) (dto.AuthResponse, error) {
	email := strings.TrimSpace(strings.ToLower(req.Email))
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
	return s.state.issueAuthResponse(ctx, user.UserID)
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
	receivedAt, err := s.state.tokens.AuthCallbackReceivedAt(context.Background(), callbackID)
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
		Received:   receivedAt > 0,
		ReceivedAt: receivedAt,
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

func (s dbAuthService) MarkCallbackReceived(callbackID string) error {
	callbackID = strings.TrimSpace(callbackID)
	if callbackID == "" {
		return ErrInvalidArgument
	}
	return s.state.tokens.MarkAuthCallbackReceived(
		context.Background(),
		callbackID,
		time.Now().UnixMilli(),
		10*time.Minute,
	)
}

func (s *dbState) issueAuthResponse(ctx context.Context, userID string) (dto.AuthResponse, error) {
	accessToken := util.OpaqueToken("access", userID)
	refreshToken := util.OpaqueToken("refresh", userID)
	if err := s.tokens.StoreAccessToken(ctx, accessToken, userID, time.Hour); err != nil {
		return dto.AuthResponse{}, err
	}
	if err := s.tokens.StoreRefreshToken(ctx, refreshToken, userID, 24*time.Hour); err != nil {
		return dto.AuthResponse{}, err
	}
	return dto.AuthResponse{
		UserID:       userID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    3600,
	}, nil
}

func (v dbTokenVerifier) Authenticate(accessToken string) (string, error) {
	userID, err := v.state.tokens.Authenticate(context.Background(), accessToken)
	if err != nil {
		return "", ErrUnauthorized
	}
	return userID, nil
}
