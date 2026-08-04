package service

import (
	"context"
	"strings"

	"github.com/slan/service-biz/internal/model"
)

func (s OpsAuthSessionService) Login(ctx context.Context, input OpsLoginInput) (OpsSessionView, error) {
	input.Email = normalizedEmail(input.Email)
	view, err := s.login(ctx, input)
	status := "success"
	if err != nil {
		status = "failure"
	}
	s.recordOperatorAuthAudit(ctx, "login", input.Email, status, input.RemoteIP, opsNow(s.Now).Unix())
	return view, err
}

func (s OpsAuthSessionService) login(ctx context.Context, input OpsLoginInput) (OpsSessionView, error) {
	operator, ok, err := s.Operators.GetOperatorByEmail(ctx, input.Email)
	if err != nil {
		return OpsSessionView{}, err
	}
	passwordHash := dummyOperatorPasswordHash
	supportedPasswordHash := ok && strings.HasPrefix(strings.TrimSpace(operator.PasswordHash), "$2")
	if supportedPasswordHash {
		passwordHash = operator.PasswordHash
	}
	passwordMatches := verifyPassword(passwordHash, input.Password)
	if !supportedPasswordHash || operator.Status != "active" || !passwordMatches {
		return OpsSessionView{}, ErrUnauthorized
	}
	now := opsNow(s.Now)
	if err := s.OperatorSessions.DeleteExpiredOperatorSessions(ctx, now.Unix()); err != nil {
		return OpsSessionView{}, err
	}
	session, err := newOperatorSession(now, s.NewOperatorSessID, operator.OperatorID)
	if err != nil {
		return OpsSessionView{}, err
	}
	if err := s.OperatorSessions.SaveOperatorSession(ctx, session); err != nil {
		return OpsSessionView{}, err
	}
	return opsSessionView(operator, session), nil
}

func (s OpsAuthSessionService) Authenticate(ctx context.Context, accessToken string) (OperatorView, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return OperatorView{}, ErrUnauthorized
	}
	session, found, err := s.OperatorSessions.GetOperatorSessionByAccessToken(ctx, accessToken)
	if err != nil {
		return OperatorView{}, err
	}
	if !found || session.ExpiresAt <= opsNow(s.Now).Unix() {
		return OperatorView{}, ErrUnauthorized
	}
	operator, found, err := s.Operators.GetOperator(ctx, session.OperatorID)
	if err != nil {
		return OperatorView{}, err
	}
	if !found || operator.Status != "active" {
		return OperatorView{}, ErrUnauthorized
	}
	return operatorView(operator), nil
}

func (s OpsAuthSessionService) Logout(ctx context.Context, accessToken string) error {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return ErrUnauthorized
	}
	err := s.OperatorSessions.DeleteOperatorSessionByAccessToken(ctx, accessToken)
	status := "success"
	if err != nil {
		status = "failure"
	}
	s.recordOperatorAuthAudit(ctx, "logout", AuthenticatedOperatorID(ctx), status, "", opsNow(s.Now).Unix())
	return err
}

func (s OpsAuthSessionService) recordOperatorAuthAudit(ctx context.Context, action, actorID, status, remoteIP string, now int64) {
	if s.Audit == nil {
		return
	}
	eventID, err := randomHex(16)
	if err != nil {
		return
	}
	actorType := "operator"
	if action == "login" {
		actorType = "operator_account"
	}
	_ = s.Audit.SaveAuditEvent(ctx, model.AuditEvent{
		EventID: "audit_" + eventID, ActorType: actorType, ActorID: strings.TrimSpace(actorID),
		Action: action, ResourceType: "operator_session", ResourceID: strings.TrimSpace(actorID),
		Status: status, RemoteIP: strings.TrimSpace(remoteIP), CreatedAt: now,
	})
}
