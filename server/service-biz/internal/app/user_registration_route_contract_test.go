package app

import (
	"testing"

	serviceapi "github.com/slan/service-biz/internal/api"
)

func TestUserCreationKeepsAppCompatibilityAndRemovesManagementRegistration(t *testing.T) {
	routes := newRouteCatalog(RouteUseCases{})
	foundAppRegister := false
	for _, route := range routeKeys(routes.app) {
		if route.path == "/api/v2/auth/register" {
			t.Fatalf("management user registration route is still exposed: %s %s", route.method, route.path)
		}
		if route.method == "POST" && route.path == "/api/app/auth/register" {
			foundAppRegister = true
		}
	}
	if !foundAppRegister {
		t.Fatal("app-compatible user registration route is missing")
	}
	foundOpsCreate := false
	for _, route := range routeKeys(routes.ops) {
		if route.method == "POST" && route.path == "/api/ops/users" {
			foundOpsCreate = true
			break
		}
	}
	if !foundOpsCreate {
		t.Fatal("operations user creation route is missing")
	}
}

type routeKey struct {
	method string
	path   string
}

func routeKeys(routes []serviceapi.Route) []routeKey {
	keys := make([]routeKey, 0, len(routes))
	for _, route := range routes {
		keys = append(keys, routeKey{method: route.Method, path: route.Path})
	}
	return keys
}
