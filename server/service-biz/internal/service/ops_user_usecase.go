package service

import (
	"context"
)

func (s OpsUserService) ListUsers(ctx context.Context) ([]OpsUserView, error) {
	users, err := s.Users.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	return buildOpsUserViews(ctx, users, s.Devices), nil
}

func (s OpsUserService) CreateUser(ctx context.Context, input CreateUserInput) (OpsUserView, error) {
	input = normalizeCreateUserInput(input)
	if input.Email == "" || normalizedSecret(input.Password) == "" {
		return OpsUserView{}, ErrInvalidArgument
	}
	if _, ok, err := s.Users.GetByEmail(ctx, input.Email); err != nil {
		return OpsUserView{}, err
	} else if ok {
		return OpsUserView{}, ErrConflict
	}
	now := opsNow(s.Now).Unix()
	user := newRegisteredUser(s.NewUserID, RegisterUserInput{Email: input.Email, Name: input.Name, Password: input.Password}, now)
	if err := s.Users.SaveUser(ctx, user); err != nil {
		return OpsUserView{}, err
	}
	return buildOpsUserView(ctx, s.Devices, user), nil
}

func (s OpsUserService) UpdateUser(ctx context.Context, input UpdateUserInput) (OpsUserView, error) {
	input = normalizeUpdateUserInput(input)
	if input.UserID == "" {
		return OpsUserView{}, ErrInvalidArgument
	}
	user, err := requireOpsUser(ctx, s.Users, input.UserID)
	if err != nil {
		return OpsUserView{}, err
	}
	user = applyUpdateUserInput(user, input, opsNow(s.Now).Unix())
	if err := s.Users.SaveUser(ctx, user); err != nil {
		return OpsUserView{}, err
	}
	return buildOpsUserView(ctx, s.Devices, user), nil
}

func (s OpsUserService) SetUserPassword(ctx context.Context, input SetUserPasswordInput) (OpsUserView, error) {
	input = normalizeSetUserPasswordInput(input)
	if input.UserID == "" || normalizedSecret(input.Password) == "" {
		return OpsUserView{}, ErrInvalidArgument
	}
	user, err := requireOpsUser(ctx, s.Users, input.UserID)
	if err != nil {
		return OpsUserView{}, err
	}
	user = applyChangedUserPassword(user, input.Password, opsNow(s.Now).Unix())
	if err := s.Users.SaveUser(ctx, user); err != nil {
		return OpsUserView{}, err
	}
	return buildOpsUserView(ctx, s.Devices, user), nil
}
