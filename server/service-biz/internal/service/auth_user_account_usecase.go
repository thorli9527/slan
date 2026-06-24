package service

import "context"

func (s AuthUserAccountService) ListUsers(ctx context.Context) ([]UserSummaryView, error) {
	items, err := s.Users.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	return userSummaryViews(items), nil
}

func (s AuthUserAccountService) GetUser(ctx context.Context, userID string) (UserView, error) {
	userID = normalizeUserID(userID)
	if userID == "" {
		return UserView{}, ErrInvalidArgument
	}
	item, err := requireAuthUser(ctx, s.Users, userID)
	if err != nil {
		return UserView{}, err
	}
	return userView(item), nil
}

func (s AuthUserAccountService) ChangeUserPassword(ctx context.Context, input ChangeUserPasswordInput) (ChangedUserPasswordView, error) {
	input = normalizeChangeUserPasswordInput(input)
	if input.UserID == "" || normalizedSecret(input.NewPassword) == "" {
		return ChangedUserPasswordView{}, ErrInvalidArgument
	}
	if input.ActorUserID != "" && input.ActorUserID != input.UserID {
		return ChangedUserPasswordView{}, ErrUnauthorized
	}
	user, err := requireAuthUser(ctx, s.Users, input.UserID)
	if err != nil {
		return ChangedUserPasswordView{}, err
	}
	if normalizedSecret(input.OldPassword) != "" && !verifyPassword(user.PasswordHash, input.OldPassword) {
		return ChangedUserPasswordView{}, ErrUnauthorized
	}
	user = applyChangedUserPassword(user, input.NewPassword, authNow(s.Now).Unix())
	if err := s.Users.SaveUser(ctx, user); err != nil {
		return ChangedUserPasswordView{}, err
	}
	return changedUserPasswordView(user), nil
}
