package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

func requireActiveConsoleLoginKey(
	ctx context.Context,
	sessions repository.UserSessionRepository,
	key string,
	now int64,
) (model.ConsoleLoginKey, error) {
	item, ok, err := sessions.GetConsoleLoginKeyByKey(ctx, key)
	if err != nil {
		return model.ConsoleLoginKey{}, err
	}
	if !ok {
		return model.ConsoleLoginKey{}, ErrNotFound
	}
	if item.Status != "active" || item.ExpiresAt < now {
		return model.ConsoleLoginKey{}, ErrConflict
	}
	return item, nil
}
