package service

import (
	"context"
)

func (s AuthAliasService) ListUserAliases(ctx context.Context, userID string) ([]UserAliasView, error) {
	items, err := s.Aliases.ListUserAliases(ctx, normalizeUserID(userID))
	if err != nil {
		return nil, err
	}
	return userAliasViews(items), nil
}

func (s AuthAliasService) UpsertUserAlias(ctx context.Context, input UpsertUserAliasInput) (UserAliasView, error) {
	input = normalizeUserAliasInput(input)
	if input.UserID == "" || input.Email == "" || input.Alias == "" {
		return UserAliasView{}, ErrInvalidArgument
	}
	if input.ActorUserID != "" && input.ActorUserID != input.UserID {
		return UserAliasView{}, ErrForbidden
	}
	if _, err := requireAuthUser(ctx, s.Users, input.UserID); err != nil {
		return UserAliasView{}, err
	}
	items, err := s.Aliases.ListUserAliases(ctx, input.UserID)
	if err != nil {
		return UserAliasView{}, err
	}
	alias := newUserAlias(input, findUserAliasCreatedAt(items, input.Email), authNow(s.Now).Unix())
	if err := s.Aliases.SaveUserAlias(ctx, alias); err != nil {
		return UserAliasView{}, err
	}
	return userAliasView(alias), nil
}
