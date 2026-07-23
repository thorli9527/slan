package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type UserSessionRepository interface {
	GetUserSessionByAccessToken(ctx context.Context, accessToken string) (model.UserSession, bool, error)
	GetUserSessionByRefreshToken(ctx context.Context, refreshToken string) (model.UserSession, bool, error)
	ListUserSessionsByUserID(ctx context.Context, userID string) ([]model.UserSession, error)
	SaveUserSession(ctx context.Context, session model.UserSession) error
	ReplaceUserSessionForClient(ctx context.Context, session model.UserSession) error
	ReplaceUserSession(ctx context.Context, oldAccessToken string, session model.UserSession) error
	DeleteUserSessionByAccessToken(ctx context.Context, accessToken string) error
	UserConsoleRepository
}

type UserConsoleRepository interface {
	GetConsoleLoginKeyByKey(ctx context.Context, key string) (model.ConsoleLoginKey, bool, error)
	SaveConsoleLoginKey(ctx context.Context, item model.ConsoleLoginKey) error
}
