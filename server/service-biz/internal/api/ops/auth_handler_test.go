package ops

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	servicepkg "github.com/slan/service-biz/internal/service"
)

type operatorPasswordHandlerTestUseCase struct {
	changed servicepkg.OpsChangePasswordInput
}

type authHandlerTestUseCase struct {
	loginInput servicepkg.OpsLoginInput
}

func (s *authHandlerTestUseCase) Login(_ context.Context, input servicepkg.OpsLoginInput) (servicepkg.OpsSessionView, error) {
	s.loginInput = input
	return servicepkg.OpsSessionView{}, nil
}

func (*authHandlerTestUseCase) Authenticate(context.Context, string) (servicepkg.OperatorView, error) {
	return servicepkg.OperatorView{}, nil
}

func (*authHandlerTestUseCase) Logout(context.Context, string) error { return nil }

func TestLoginPassesConnectionIPToAuditInput(t *testing.T) {
	sessions := &authHandlerTestUseCase{}
	handler := AuthHandler{OpsAuthSessions: sessions}
	request := httptest.NewRequest(http.MethodPost, "/api/ops/auth/login", strings.NewReader(`{"email":"operator@example.com","password":"secret"}`))
	request.Header.Set("Content-Type", "application/json")
	request.RemoteAddr = "203.0.113.10:43210"
	response := httptest.NewRecorder()

	handler.OpsLogin(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", response.Code, http.StatusOK)
	}
	if sessions.loginInput.RemoteIP != "203.0.113.10" {
		t.Fatalf("login remote IP = %q, want connection IP", sessions.loginInput.RemoteIP)
	}
}

func (s *operatorPasswordHandlerTestUseCase) SetOperatorPassword(context.Context, servicepkg.SetOperatorPasswordInput) (servicepkg.OpsOperatorView, error) {
	return servicepkg.OpsOperatorView{}, nil
}

func (s *operatorPasswordHandlerTestUseCase) ChangePassword(_ context.Context, input servicepkg.OpsChangePasswordInput) (servicepkg.OpsOperatorView, error) {
	s.changed = input
	return servicepkg.OpsOperatorView{}, nil
}

func TestChangePasswordUsesAuthenticatedOperatorID(t *testing.T) {
	passwords := &operatorPasswordHandlerTestUseCase{}
	handler := AuthHandler{OpsOperatorPasswords: passwords}
	request := httptest.NewRequest(http.MethodPatch, "/api/ops/auth/password", strings.NewReader(`{"operatorId":"operator-victim","oldPassword":"old-secret","newPassword":"new-secret"}`))
	request.Header.Set("Content-Type", "application/json")
	request = request.WithContext(servicepkg.WithAuthenticatedOperator(request.Context(), "operator-current"))
	response := httptest.NewRecorder()
	handler.OpsChangePassword(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("change password status = %d, want %d", response.Code, http.StatusOK)
	}
	if passwords.changed.OperatorID != "operator-current" {
		t.Fatalf("changed operator = %q, want current operator", passwords.changed.OperatorID)
	}
	if passwords.changed.OldPassword != "old-secret" || passwords.changed.Password != "new-secret" {
		t.Fatalf("unexpected password input: %#v", passwords.changed)
	}
}
