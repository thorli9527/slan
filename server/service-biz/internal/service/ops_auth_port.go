package service

import "context"

type OpsAuthSessionUseCase interface {
	Login(ctx context.Context, input OpsLoginInput) (OpsSessionView, error)
	Authenticate(ctx context.Context, accessToken string) (OpsSessionView, error)
}

type OpsOperatorUseCase interface {
	ListOperators(ctx context.Context) ([]OpsOperatorView, error)
	CreateOperator(ctx context.Context, input CreateOperatorInput) (OpsOperatorView, error)
	UpdateOperator(ctx context.Context, input UpdateOperatorInput) (OpsOperatorView, error)
}

type OpsOperatorPasswordUseCase interface {
	SetOperatorPassword(ctx context.Context, input SetOperatorPasswordInput) (OpsOperatorView, error)
	ChangePassword(ctx context.Context, input OpsChangePasswordInput) (OpsOperatorView, error)
}
