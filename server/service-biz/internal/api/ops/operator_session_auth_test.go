package ops

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type operatorSessionAuthStub struct{}

func (operatorSessionAuthStub) Login(context.Context, servicepkg.OpsLoginInput) (servicepkg.OpsSessionView, error) {
	return servicepkg.OpsSessionView{}, nil
}

func (operatorSessionAuthStub) Authenticate(_ context.Context, token string) (servicepkg.OpsSessionView, error) {
	if token != "valid-token" {
		return servicepkg.OpsSessionView{}, servicepkg.ErrUnauthorized
	}
	return servicepkg.OpsSessionView{Operator: servicepkg.OperatorView{OperatorID: "operator-1"}}, nil
}

func TestRequiredOperatorSessionProtectsOpsRoutesAndLeavesLoginPublic(t *testing.T) {
	routes := withRequiredOperatorSession([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodPost, "/api/ops/auth/login", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
		serviceapi.NewRoute(http.MethodGet, "/api/ops/networks", func(w http.ResponseWriter, r *http.Request) {
			if got := serviceapi.AuthenticatedOperatorID(r.Context()); got != "operator-1" {
				t.Fatalf("expected authenticated operator in context, got %q", got)
			}
			w.WriteHeader(http.StatusNoContent)
		}),
	}, operatorSessionAuthStub{})

	login := findRouteHandler(t, routes, "POST /api/ops/auth/login")
	loginResponse := httptest.NewRecorder()
	login(loginResponse, httptest.NewRequest(http.MethodPost, "/api/ops/auth/login", nil))
	if loginResponse.Code != http.StatusNoContent {
		t.Fatalf("login must remain public, got HTTP %d", loginResponse.Code)
	}

	protected := findRouteHandler(t, routes, "GET /api/ops/networks")
	unauthorized := httptest.NewRecorder()
	protected(unauthorized, httptest.NewRequest(http.MethodGet, "/api/ops/networks", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("missing token must return 401, got HTTP %d", unauthorized.Code)
	}
	authorizedRequest := httptest.NewRequest(http.MethodGet, "/api/ops/networks", nil)
	authorizedRequest.Header.Set("Authorization", "Bearer valid-token")
	authorized := httptest.NewRecorder()
	protected(authorized, authorizedRequest)
	if authorized.Code != http.StatusNoContent {
		t.Fatalf("valid token must reach handler, got HTTP %d", authorized.Code)
	}
}

func findRouteHandler(t *testing.T, routes []serviceapi.Route, key string) http.HandlerFunc {
	t.Helper()
	for _, route := range routes {
		if route.Key() == key {
			return route.Handler
		}
	}
	t.Fatalf("route not found: %s", key)
	return nil
}
