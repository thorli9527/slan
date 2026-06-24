package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type UserSessionRepository interface {
	GetUserSessionByAccessToken(ctx context.Context, accessToken string) (model.UserSession, bool, error)
	SaveUserSession(ctx context.Context, session model.UserSession) error
	DeleteUserSessionByAccessToken(ctx context.Context, accessToken string) error
	UserConsoleRepository
}

type UserConsoleRepository interface {
	GetConsoleLoginKeyByKey(ctx context.Context, key string) (model.ConsoleLoginKey, bool, error)
	SaveConsoleLoginKey(ctx context.Context, item model.ConsoleLoginKey) error
}
