package service

import "context"

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
	network := newDefaultUserNetwork(s.NewNetID, user.UserID, now.Unix())
	if err := s.Networks.SaveNetwork(ctx, network); err != nil {
		return AuthSessionView{}, err
	}
	securityGroup := newDefaultUserSecurityGroup(s.NewSecurityGroupID, s.Networks, network.NetworkID, now.Unix())
	if err := s.Networks.SaveSecurityGroup(ctx, securityGroup); err != nil {
		return AuthSessionView{}, err
	}
	session, err := newAuthUserSession(now, s.NewSessID, user.UserID)
	if err != nil {
		return AuthSessionView{}, err
	}
	if err := s.Sessions.SaveUserSession(ctx, session); err != nil {
		return AuthSessionView{}, err
	}
	return authSessionView(user, session), nil
}
