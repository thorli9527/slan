package service

import (
	"context"
)

func (s AuthUserRegistrationService) RegisterUser(ctx context.Context, input RegisterUserInput) (AuthSessionView, error) {
	input = normalizeRegisterUserInput(input)
	if input.Email == "" || normalizedSecret(input.Password) == "" {
		return AuthSessionView{}, ErrInvalidArgument
	}
	if _, ok, _ := s.Users.GetByEmail(ctx, input.Email); ok {
		return AuthSessionView{}, ErrConflict
	}
	now := authNow(s.Now)
	user := newRegisteredUser(s.NewUserID, input, now.Unix())
	if err := s.Users.SaveUser(ctx, user); err != nil {
		return AuthSessionView{}, err
	}
	session, err := newAuthUserSession(now, s.NewSessID, user.UserID, tokenModeShort, input.ClientType, input.DeviceID)
	if err != nil {
		return AuthSessionView{}, err
	}
	if err := s.Sessions.ReplaceUserSessionForClient(ctx, session); err != nil {
		return AuthSessionView{}, err
	}
	return authSessionView(user, session), nil
}
