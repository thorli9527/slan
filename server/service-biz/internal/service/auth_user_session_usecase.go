package service

import "context"

func (s AuthUserSessionService) LoginUser(ctx context.Context, input LoginUserInput) (AuthSessionView, error) {
	input = normalizeLoginUserInput(input)
	user, err := requireUserLogin(ctx, s.Users, input)
	if err != nil {
		return AuthSessionView{}, err
	}
	session, err := newAuthUserSession(authNow(s.Now), s.NewSessID, user.UserID, input.SessionMode)
	if err != nil {
		return AuthSessionView{}, err
	}
	if err := replaceUserSession(ctx, s.Sessions, session); err != nil {
		return AuthSessionView{}, err
	}
	return authSessionView(user, session), nil
}

func (s AuthUserSessionService) GetUserSession(ctx context.Context, accessToken string) (AuthSessionView, error) {
	now := authNow(s.Now)
	session, err := requireActiveUserSession(ctx, s.Sessions, normalizeUserAccessToken(accessToken), now.Unix())
	if err != nil {
		return AuthSessionView{}, err
	}
	user, err := requireAuthUser(ctx, s.Users, session.UserID)
	if err != nil {
		return AuthSessionView{}, err
	}
	return authSessionView(user, session), nil
}

func (s AuthUserSessionService) RenewUserSession(ctx context.Context, accessToken string, input RenewUserSessionInput) (AuthSessionView, error) {
	input = normalizeRenewUserSessionInput(input)
	if input.RefreshToken == "" {
		return AuthSessionView{}, ErrInvalidArgument
	}
	now := authNow(s.Now)
	session, ok, err := s.Sessions.GetUserSessionByRefreshToken(ctx, input.RefreshToken)
	if err != nil {
		return AuthSessionView{}, err
	}
	if !ok || session.Status != tokenStatusActive || session.RevokedAt > 0 || session.RefreshExpiry < now.Unix() {
		return AuthSessionView{}, ErrUnauthorized
	}
	user, err := requireAuthUser(ctx, s.Users, session.UserID)
	if err != nil {
		return AuthSessionView{}, err
	}
	if accessToken != "" && normalizeUserAccessToken(accessToken) != session.AccessToken {
		return AuthSessionView{}, ErrUnauthorized
	}
	nextSession, err := newAuthUserSession(now, s.NewSessID, session.UserID, session.SessionMode)
	if err != nil {
		return AuthSessionView{}, err
	}
	if err := replaceUserSession(ctx, s.Sessions, nextSession); err != nil {
		return AuthSessionView{}, err
	}
	return authSessionView(user, nextSession), nil
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
