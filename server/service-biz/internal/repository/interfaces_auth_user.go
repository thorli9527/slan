package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type UserRepository interface {
	ListUsers(ctx context.Context) ([]model.User, error)
	GetUser(ctx context.Context, userID string) (model.User, bool, error)
	GetByEmail(ctx context.Context, email string) (model.User, bool, error)
	SaveUser(ctx context.Context, user model.User) error
}
