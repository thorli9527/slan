package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
)

func (s AuthUserSessionService) LoginUser(ctx context.Context, input LoginUserInput) (AuthSessionView, error) {
	input = normalizeLoginUserInput(input)
	if input.ClientType == UserSessionClientWeb {
		input.SessionMode = tokenModeShort
	}
	user, err := requireUserLogin(ctx, s.Users, input)
	if err != nil {
		return AuthSessionView{}, err
	}
	session, err := newAuthUserSession(authNow(s.Now), s.NewSessID, user.UserID, input.SessionMode, input.ClientType, input.DeviceID)
	if err != nil {
		return AuthSessionView{}, err
	}
	if err := s.Sessions.ReplaceUserSessionForClient(ctx, session); err != nil {
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
	digest := sha256.Sum256([]byte(input.RefreshToken))
	refreshTokenHash := hex.EncodeToString(digest[:])
	rotationRetry := session.RefreshToken != input.RefreshToken
	if rotationRetry && (session.PreviousRefreshTokenHash != refreshTokenHash || session.RefreshRotationGraceExpiry < now.Unix()) {
		return AuthSessionView{}, ErrUnauthorized
	}
	user, err := requireAuthUser(ctx, s.Users, session.UserID)
	if err != nil {
		return AuthSessionView{}, err
	}
	if !rotationRetry && accessToken != "" && normalizeUserAccessToken(accessToken) != session.AccessToken {
		return AuthSessionView{}, ErrUnauthorized
	}
	if rotationRetry {
		return authSessionView(user, session), nil
	}
	nextSession, err := newAuthUserSession(now, s.NewSessID, session.UserID, session.SessionMode, session.ClientType, session.DeviceID)
	if err != nil {
		return AuthSessionView{}, err
	}
	nextSession.PreviousRefreshTokenHash = refreshTokenHash
	nextSession.RefreshRotationGraceExpiry = now.Add(userRefreshRotationGrace).Unix()
	if err := s.Sessions.ReplaceUserSession(ctx, session.AccessToken, nextSession); err != nil {
		return AuthSessionView{}, err
	}
	return authSessionView(user, nextSession), nil
}

func (s AuthUserSessionService) LogoutUser(ctx context.Context, accessToken string, _ LogoutUserInput) error {
	return s.Sessions.DeleteUserSessionByAccessToken(ctx, normalizeUserAccessToken(accessToken))
}
