package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type userSessionAuthStub struct{}

func (userSessionAuthStub) LoginUser(context.Context, servicepkg.LoginUserInput) (servicepkg.AuthSessionView, error) {
	return servicepkg.AuthSessionView{}, servicepkg.ErrUnauthorized
}

func (userSessionAuthStub) GetUserSession(_ context.Context, token string) (servicepkg.AuthSessionView, error) {
	if token != "valid-token" {
		return servicepkg.AuthSessionView{}, servicepkg.ErrUnauthorized
	}
	return servicepkg.AuthSessionView{User: servicepkg.UserView{UserID: "authenticated-user"}}, nil
}

func (userSessionAuthStub) RenewUserSession(context.Context, string, servicepkg.RenewUserSessionInput) (servicepkg.AuthSessionView, error) {
	return servicepkg.AuthSessionView{}, servicepkg.ErrUnauthorized
}

func (userSessionAuthStub) LogoutUser(context.Context, string, servicepkg.LogoutUserInput) error {
	return nil
}

func TestRequiredUserSessionRejectsMissingToken(t *testing.T) {
	routes := withRequiredUserSession([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/protected", func(http.ResponseWriter, *http.Request) {
			t.Fatal("protected handler must not run")
		}),
	}, userSessionAuthStub{})

	response := httptest.NewRecorder()
	routes[0].Handler(response, httptest.NewRequest(http.MethodGet, "/protected", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", response.Code)
	}
}

func TestRequiredUserSessionInjectsAuthenticatedUser(t *testing.T) {
	routes := withRequiredUserSession([]serviceapi.Route{
		serviceapi.NewRoute(http.MethodGet, "/protected", func(w http.ResponseWriter, r *http.Request) {
			if got := requestActorUserID(r); got != "authenticated-user" {
				t.Fatalf("expected authenticated actor, got %q", got)
			}
			if got := requestUserID(r); got != "authenticated-user" {
				t.Fatalf("expected authenticated user, got %q", got)
			}
			w.WriteHeader(http.StatusNoContent)
		}),
	}, userSessionAuthStub{})

	request := httptest.NewRequest(http.MethodGet, "/protected?actorUserId=spoofed&userId=spoofed", nil)
	request.Header.Set("Authorization", "Bearer valid-token")
	response := httptest.NewRecorder()
	routes[0].Handler(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("expected protected handler response, got %d", response.Code)
	}
}
