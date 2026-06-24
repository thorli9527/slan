package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type UserAliasRepository interface {
	ListUserAliases(ctx context.Context, userID string) ([]model.UserAlias, error)
	SaveUserAlias(ctx context.Context, alias model.UserAlias) error
}
