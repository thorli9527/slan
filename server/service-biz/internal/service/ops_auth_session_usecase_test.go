package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
)

type opsAuthTestStore struct {
	operator model.Operator
	session  model.OperatorSession
}

func mustHashOperatorPassword(t *testing.T, password string) string {
	t.Helper()
	hash, err := hashPassword(password)
	if err != nil {
		t.Fatalf("hashPassword returned error: %v", err)
	}
	return hash
}

func (s *opsAuthTestStore) ListOperators(context.Context) ([]model.Operator, error) {
	return []model.Operator{s.operator}, nil
}

func (s *opsAuthTestStore) GetOperator(_ context.Context, operatorID string) (model.Operator, bool, error) {
	return s.operator, s.operator.OperatorID == operatorID, nil
}

func (s *opsAuthTestStore) GetOperatorByEmail(_ context.Context, email string) (model.Operator, bool, error) {
	return s.operator, s.operator.Email == email, nil
}

func (s *opsAuthTestStore) SaveOperator(_ context.Context, operator model.Operator) error {
	s.operator = operator
	return nil
}

func (s *opsAuthTestStore) GetOperatorSessionByAccessToken(_ context.Context, token string) (model.OperatorSession, bool, error) {
	return s.session, s.session.AccessToken == token, nil
}

func (s *opsAuthTestStore) SaveOperatorSession(_ context.Context, session model.OperatorSession) error {
	s.session = session
	return nil
}

func (s *opsAuthTestStore) DeleteOperatorSessionsByOperatorID(_ context.Context, operatorID string) error {
	if s.session.OperatorID == operatorID {
		s.session = model.OperatorSession{}
	}
	return nil
}

func (s *opsAuthTestStore) DeleteOperatorSessionByAccessToken(_ context.Context, accessToken string) error {
	if s.session.AccessToken == accessToken {
		s.session = model.OperatorSession{}
	}
	return nil
}

func (s *opsAuthTestStore) DeleteExpiredOperatorSessions(_ context.Context, now int64) error {
	if s.session.SessionID != "" && s.session.ExpiresAt <= now {
		s.session = model.OperatorSession{}
	}
	return nil
}

func TestOpsAuthenticationRejectsDisabledOperator(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	store := &opsAuthTestStore{
		operator: model.Operator{
			OperatorID: "operator-1", Email: "operator@example.com", PasswordHash: mustHashOperatorPassword(t, "strong-secret-1"), Status: "disabled",
		},
		session: model.OperatorSession{
			SessionID: "session-1", OperatorID: "operator-1", AccessToken: "access-1", ExpiresAt: now.Add(time.Hour).Unix(),
		},
	}
	service := OpsAuthSessionService{Operators: store, OperatorSessions: store, Now: func() time.Time { return now }}

	if _, err := service.Login(context.Background(), OpsLoginInput{Email: store.operator.Email, Password: "strong-secret-1"}); err != ErrUnauthorized {
		t.Fatalf("disabled operator login error = %v, want %v", err, ErrUnauthorized)
	}
	if _, err := service.Authenticate(context.Background(), "access-1"); err != ErrUnauthorized {
		t.Fatalf("disabled operator session error = %v, want %v", err, ErrUnauthorized)
	}
}

func TestOpsAuthenticationRejectsLegacyPasswordRecords(t *testing.T) {
	store := &opsAuthTestStore{operator: model.Operator{
		OperatorID: "operator-1", Email: "operator@example.com", PasswordHash: "plain:legacy-secret", Status: "active",
	}}
	service := OpsAuthSessionService{Operators: store, OperatorSessions: store}
	if _, err := service.Login(context.Background(), OpsLoginInput{Email: store.operator.Email, Password: "legacy-secret"}); err != ErrUnauthorized {
		t.Fatalf("legacy operator password error = %v, want %v", err, ErrUnauthorized)
	}
}

func TestChangingOperatorPasswordRevokesExistingSessions(t *testing.T) {
	store := &opsAuthTestStore{
		operator: model.Operator{OperatorID: "operator-1", Email: "operator@example.com", Status: "active"},
		session:  model.OperatorSession{SessionID: "session-1", OperatorID: "operator-1", AccessToken: "access-1"},
	}
	audit := &deviceCredentialTestAudit{}
	service := OpsOperatorPasswordService{Operators: store, OperatorSessions: store, Audit: audit}
	ctx := WithAuthenticatedOperator(context.Background(), "operator-admin")
	if _, err := service.SetOperatorPassword(ctx, SetOperatorPasswordInput{
		OperatorID: "operator-1", Password: "new-strong-secret",
	}); err != nil {
		t.Fatalf("SetOperatorPassword returned error: %v", err)
	}
	if store.session.SessionID != "" {
		t.Fatalf("password change left session active: %#v", store.session)
	}
	if len(audit.events) != 1 || audit.events[0].Action != "reset_password" || audit.events[0].Status != "success" ||
		audit.events[0].ActorID != "operator-admin" || audit.events[0].ResourceID != "operator-1" {
		t.Fatalf("unexpected reset password audit: %#v", audit.events)
	}
}

func TestChangePasswordRequiresCurrentPassword(t *testing.T) {
	store := &opsAuthTestStore{operator: model.Operator{
		OperatorID: "operator-1", PasswordHash: mustHashOperatorPassword(t, "old-strong-secret"), Status: "active",
	}}
	audit := &deviceCredentialTestAudit{}
	service := OpsOperatorPasswordService{Operators: store, OperatorSessions: store, Audit: audit}
	ctx := WithAuthenticatedOperator(context.Background(), "operator-1")
	if _, err := service.ChangePassword(ctx, OpsChangePasswordInput{
		OperatorID: "operator-1", OldPassword: "wrong-secret", Password: "new-strong-secret",
	}); err != ErrUnauthorized {
		t.Fatalf("wrong current password error = %v, want %v", err, ErrUnauthorized)
	}
	if _, err := service.ChangePassword(ctx, OpsChangePasswordInput{
		OperatorID: "operator-1", OldPassword: "old-strong-secret", Password: "new-strong-secret",
	}); err != nil {
		t.Fatalf("correct current password returned error: %v", err)
	}
	if !verifyPassword(store.operator.PasswordHash, "new-strong-secret") {
		t.Fatal("new password was not stored")
	}
	if len(audit.events) != 2 || audit.events[0].Action != "change_password" || audit.events[0].Status != "failure" ||
		audit.events[1].Action != "change_password" || audit.events[1].Status != "success" {
		t.Fatalf("unexpected change password audit: %#v", audit.events)
	}
}

func TestDisablingOperatorRevokesExistingSessions(t *testing.T) {
	store := &opsAuthTestStore{
		operator: model.Operator{OperatorID: "operator-1", Status: "active"},
		session:  model.OperatorSession{SessionID: "session-1", OperatorID: "operator-1", AccessToken: "access-1"},
	}
	audit := &deviceCredentialTestAudit{}
	service := OpsOperatorService{Operators: store, OperatorSessions: store, Audit: audit}
	ctx := WithAuthenticatedOperator(context.Background(), "operator-admin")
	if _, err := service.UpdateOperator(ctx, UpdateOperatorInput{
		OperatorID: "operator-1", Status: "disabled",
	}); err != nil {
		t.Fatalf("UpdateOperator returned error: %v", err)
	}
	if store.session.SessionID != "" {
		t.Fatalf("disabled operator retained session: %#v", store.session)
	}
	if len(audit.events) != 1 || audit.events[0].Action != "update_operator" || audit.events[0].Status != "success" ||
		audit.events[0].ActorID != "operator-admin" || audit.events[0].ResourceID != "operator-1" {
		t.Fatalf("unexpected operator update audit: %#v", audit.events)
	}
}

func TestCreatingOperatorWritesAuditWithoutPasswordMaterial(t *testing.T) {
	store := &opsAuthTestStore{}
	audit := &deviceCredentialTestAudit{}
	service := OpsOperatorService{Operators: store, Audit: audit, Now: func() time.Time { return time.Unix(1_700_000_000, 0) }}
	ctx := WithAuthenticatedOperator(context.Background(), "operator-admin")

	view, err := service.CreateOperator(ctx, CreateOperatorInput{
		Email: "new-operator@example.com", Name: "New Operator", Role: "operator", Password: "correct-horse-battery-staple",
	})
	if err != nil {
		t.Fatal(err)
	}
	if view.Operator.OperatorID == "" || !strings.HasPrefix(store.operator.PasswordHash, "$2") {
		t.Fatalf("unexpected created operator: view=%+v stored=%+v", view, store.operator)
	}
	if len(audit.events) != 1 || audit.events[0].Action != "create_operator" || audit.events[0].Status != "success" ||
		audit.events[0].ActorID != "operator-admin" || audit.events[0].ResourceID != view.Operator.OperatorID || audit.events[0].Detail != "" {
		t.Fatalf("unexpected create operator audit: %#v", audit.events)
	}
}

func TestOpsLogoutRevokesCurrentSessionOnly(t *testing.T) {
	store := &opsAuthTestStore{session: model.OperatorSession{
		SessionID: "session-1", OperatorID: "operator-1", AccessToken: "access-1",
	}}
	service := OpsAuthSessionService{OperatorSessions: store}
	if err := service.Logout(context.Background(), "access-1"); err != nil {
		t.Fatalf("Logout returned error: %v", err)
	}
	if store.session.SessionID != "" {
		t.Fatalf("logout retained current session: %#v", store.session)
	}
}

func TestOperatorUpdateRejectsRemovingLastActiveAdmin(t *testing.T) {
	store := &opsAuthTestStore{operator: model.Operator{
		OperatorID: "operator-1", Role: "admin", Status: "active",
	}}
	service := OpsOperatorService{Operators: store, OperatorSessions: store}
	if _, err := service.UpdateOperator(context.Background(), UpdateOperatorInput{
		OperatorID: "operator-1", Role: "operator",
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("last admin demotion error = %v, want %v", err, ErrConflict)
	}
	if _, err := service.UpdateOperator(context.Background(), UpdateOperatorInput{
		OperatorID: "operator-1", Status: "disabled",
	}); !errors.Is(err, ErrConflict) {
		t.Fatalf("last admin disable error = %v, want %v", err, ErrConflict)
	}
}

func TestOperatorRoleAndStatusValidation(t *testing.T) {
	store := &opsAuthTestStore{operator: model.Operator{OperatorID: "operator-1", Role: "operator", Status: "active"}}
	service := OpsOperatorService{Operators: store, OperatorSessions: store}
	if _, err := service.UpdateOperator(context.Background(), UpdateOperatorInput{
		OperatorID: "operator-1", Role: "super_admin",
	}); err != ErrInvalidArgument {
		t.Fatalf("invalid role error = %v, want %v", err, ErrInvalidArgument)
	}
	if _, err := service.UpdateOperator(context.Background(), UpdateOperatorInput{
		OperatorID: "operator-1", Status: "pending",
	}); err != ErrInvalidArgument {
		t.Fatalf("invalid status error = %v, want %v", err, ErrInvalidArgument)
	}
}

func TestOpsLoginCleansExpiredSessionsAndWritesAudit(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	store := &opsAuthTestStore{
		operator: model.Operator{
			OperatorID: "operator-1", Email: "operator@example.com", Status: "active",
			PasswordHash: mustHashOperatorPassword(t, "correct-horse-battery-staple"),
		},
		session: model.OperatorSession{
			SessionID: "expired-session", OperatorID: "operator-1", AccessToken: "expired-token", ExpiresAt: now.Unix(),
		},
	}
	audit := &deviceCredentialTestAudit{}
	service := OpsAuthSessionService{
		Operators: store, OperatorSessions: store, Audit: audit,
		NewOperatorSessID: func(string) string { return "session-new" }, Now: func() time.Time { return now },
	}
	view, err := service.Login(context.Background(), OpsLoginInput{
		Email: " Operator@Example.COM ", Password: "correct-horse-battery-staple", RemoteIP: "203.0.113.10",
	})
	if err != nil {
		t.Fatalf("Login returned error: %v", err)
	}
	if view.Session.SessionID != "session-new" || store.session.SessionID != "session-new" {
		t.Fatalf("expired session was not replaced: view=%#v stored=%#v", view.Session, store.session)
	}
	if len(audit.events) != 1 || audit.events[0].Action != "login" || audit.events[0].Status != "success" || audit.events[0].ActorID != "operator@example.com" || audit.events[0].RemoteIP != "203.0.113.10" {
		t.Fatalf("unexpected login audit: %#v", audit.events)
	}

	if _, err := service.Login(context.Background(), OpsLoginInput{Email: "missing@example.com", Password: "wrong-password", RemoteIP: "198.51.100.20"}); err != ErrUnauthorized {
		t.Fatalf("invalid login error = %v, want %v", err, ErrUnauthorized)
	}
	if len(audit.events) != 2 || audit.events[1].Status != "failure" || audit.events[1].ActorID != "missing@example.com" || audit.events[1].RemoteIP != "198.51.100.20" {
		t.Fatalf("unexpected failed login audit: %#v", audit.events)
	}
}
