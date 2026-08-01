package service

import "context"

type OpsUserUseCase interface {
	ListUsers(ctx context.Context) ([]OpsUserView, error)
	CreateUser(ctx context.Context, input CreateUserInput) (OpsUserView, error)
	UpdateUser(ctx context.Context, input UpdateUserInput) (OpsUserView, error)
	SetUserPassword(ctx context.Context, input SetUserPasswordInput) (OpsUserView, error)
}
