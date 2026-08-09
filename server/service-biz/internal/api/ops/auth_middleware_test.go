package ops

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	servicepkg "github.com/slan/service-biz/internal/service"
)

type operatorAuthMiddlewareTestUseCase struct {
	role string
}

func (operatorAuthMiddlewareTestUseCase) Login(context.Context, servicepkg.OpsLoginInput) (servicepkg.OpsSessionView, error) {
	return servicepkg.OpsSessionView{}, nil
}

func (operatorAuthMiddlewareTestUseCase) Logout(context.Context, string) error { return nil }

func (s operatorAuthMiddlewareTestUseCase) Authenticate(_ context.Context, token string) (servicepkg.OperatorView, error) {
	if token != "valid-token" {
		return servicepkg.OperatorView{}, servicepkg.ErrUnauthorized
	}
	return servicepkg.OperatorView{OperatorID: "operator-1", Role: s.role, Status: "active"}, nil
}

func TestOperatorAuthMiddlewareProtectsManagementRoutes(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ops/auth/login" && servicepkg.AuthenticatedOperatorID(r.Context()) != "operator-1" {
			t.Fatalf("authenticated operator missing from context")
		}
		w.WriteHeader(http.StatusNoContent)
	})
	handler := WithOperatorAuth(next, operatorAuthMiddlewareTestUseCase{role: "operator"})

	login := httptest.NewRecorder()
	handler.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/api/ops/auth/login", nil))
	if login.Code != http.StatusNoContent {
		t.Fatalf("login status = %d, want %d", login.Code, http.StatusNoContent)
	}

	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/ops/device-credentials", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want %d", unauthorized.Code, http.StatusUnauthorized)
	}

	authorizedRequest := httptest.NewRequest(http.MethodGet, "/api/ops/device-credentials", nil)
	authorizedRequest.Header.Set("Authorization", "Bearer valid-token")
	authorized := httptest.NewRecorder()
	handler.ServeHTTP(authorized, authorizedRequest)
	if authorized.Code != http.StatusNoContent {
		t.Fatalf("authorized status = %d, want %d", authorized.Code, http.StatusNoContent)
	}
}

func TestOperatorAuthMiddlewareRequiresAdminForOperatorManagement(t *testing.T) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("non-admin request reached operator management handler")
	})
	handler := WithOperatorAuth(next, operatorAuthMiddlewareTestUseCase{role: "operator"})
	request := httptest.NewRequest(http.MethodPost, "/api/ops/operators", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("operator management status = %d, want %d", response.Code, http.StatusForbidden)
	}
}

func TestOperatorAuthMiddlewareRequiresAdminForServerNodeDeployment(t *testing.T) {
	if !operatorAdminPath("/api/ops/server-nodes/server123/deploy") {
		t.Fatal("server node deployment must require admin")
	}
	if !operatorAdminPath("/api/opt/server-nodes") {
		t.Fatal("legacy ops alias must require admin")
	}
}
