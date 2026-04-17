package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
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

	userID := newID("user")
	if err := s.state.pg.CreateUser(ctx, repo.User{
		UserID:       userID,
		Email:        email,
		PasswordHash: hashPassword(req.Password),
	}); err != nil {
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
	if user.PasswordHash != hashPassword(req.Password) {
		return dto.AuthResponse{}, ErrUnauthorized
	}
	return s.state.issueAuthResponse(ctx, user.UserID)
}
