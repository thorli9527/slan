package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func replaceUserSession(
	ctx context.Context,
	sessions repository.UserSessionRepository,
	next model.UserSession,
) error {
	items, err := sessions.ListUserSessionsByUserID(ctx, next.UserID)
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.AccessToken == "" {
			continue
		}
		if err := sessions.DeleteUserSessionByAccessToken(ctx, item.AccessToken); err != nil {
			return err
		}
	}
	return sessions.SaveUserSession(ctx, next)
}
