package service

import "context"

func (s AuthUserSessionService) LoginUser(ctx context.Context, input LoginUserInput) (AuthSessionView, error) {
	input = normalizeLoginUserInput(input)
	user, err := requireUserLogin(ctx, s.Users, input)
	if err != nil {
		return AuthSessionView{}, err
	}
	session, err := newAuthUserSession(authNow(s.Now), s.NewSessID, user.UserID)
	if err != nil {
		return AuthSessionView{}, err
	}
	if err := s.Sessions.SaveUserSession(ctx, session); err != nil {
		return AuthSessionView{}, err
	}
	return authSessionView(user, session), nil
}

func (s AuthUserSessionService) RenewUserSession(ctx context.Context, accessToken string) (AuthSessionView, error) {
	session, err := requireActiveUserSession(ctx, s.Sessions, normalizeUserAccessToken(accessToken), authNow(s.Now).Unix())
	if err != nil {
		return AuthSessionView{}, err
	}
	user, err := requireAuthUser(ctx, s.Users, session.UserID)
	if err != nil {
		return AuthSessionView{}, err
	}
	return authSessionView(user, session), nil
}

func (s AuthUserSessionService) LogoutUser(ctx context.Context, accessToken string, input LogoutUserInput) error {
	if err := s.Sessions.DeleteUserSessionByAccessToken(ctx, normalizeUserAccessToken(accessToken)); err != nil {
		return err
	}
	if token := normalizeDeviceAccessToken(input.DeviceToken); token != "" {
		if err := s.Devices.DeleteDeviceSessionByAccessToken(ctx, token); err != nil {
			return err
		}
	}
	return nil
}
