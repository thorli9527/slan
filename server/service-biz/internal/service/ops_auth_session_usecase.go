package service

import (
	"context"
	"strings"
)

func (s OpsAuthSessionService) Login(ctx context.Context, input OpsLoginInput) (OpsSessionView, error) {
	operator, ok, err := s.Operators.GetOperatorByEmail(ctx, input.Email)
	if err != nil {
		return OpsSessionView{}, err
	}
	if !ok || !verifyPassword(operator.PasswordHash, input.Password) {
		return OpsSessionView{}, ErrUnauthorized
	}
	session, err := newOperatorSession(opsNow(s.Now), s.NewOperatorSessID, operator.OperatorID)
	if err != nil {
		return OpsSessionView{}, err
	}
	if err := s.OperatorSessions.SaveOperatorSession(ctx, session); err != nil {
		return OpsSessionView{}, err
	}
	return opsSessionView(operator, session), nil
}

func (s OpsAuthSessionService) Authenticate(ctx context.Context, accessToken string) (OpsSessionView, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return OpsSessionView{}, ErrUnauthorized
	}
	session, ok, err := s.OperatorSessions.GetOperatorSessionByAccessToken(ctx, accessToken)
	if err != nil {
		return OpsSessionView{}, err
	}
	if !ok || session.ExpiresAt <= opsNow(s.Now).Unix() {
		return OpsSessionView{}, ErrUnauthorized
	}
	operator, ok, err := s.Operators.GetOperator(ctx, session.OperatorID)
	if err != nil {
		return OpsSessionView{}, err
	}
	if !ok || strings.TrimSpace(operator.Status) != "active" {
		return OpsSessionView{}, ErrUnauthorized
	}
	return opsSessionView(operator, session), nil
}
