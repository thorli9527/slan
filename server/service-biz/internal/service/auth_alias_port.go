package service

import "context"

type AuthAliasUseCase interface {
	ListUserAliases(ctx context.Context, userID string) ([]UserAliasView, error)
	UpsertUserAlias(ctx context.Context, input UpsertUserAliasInput) (UserAliasView, error)
}
