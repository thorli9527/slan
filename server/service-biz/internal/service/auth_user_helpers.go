package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func requireUserLogin(ctx context.Context, users repository.UserRepository, input LoginUserInput) (model.User, error) {
	user, ok, err := users.GetByEmail(ctx, input.Email)
	if err != nil {
		return model.User{}, err
	}
	if !ok || !verifyPassword(user.PasswordHash, input.Password) {
		return model.User{}, ErrUnauthorized
	}
	return user, nil
}

func requireActiveUserSession(
	ctx context.Context,
	sessions repository.UserSessionRepository,
	accessToken string,
	now int64,
) (model.UserSession, error) {
	session, ok, err := sessions.GetUserSessionByAccessToken(ctx, accessToken)
	if err != nil {
		return model.UserSession{}, err
	}
	if !ok || session.ExpiresAt < now {
		return model.UserSession{}, ErrUnauthorized
	}
	return session, nil
}
