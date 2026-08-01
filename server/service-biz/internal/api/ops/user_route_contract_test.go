package ops

import (
	"strings"
	"testing"
)

func TestUserHandlerPublishesGlobalUserRoutesOnly(t *testing.T) {
	routes := UserHandler{}.Routes()
	want := map[string]bool{
		"GET /api/ops/users":                     false,
		"POST /api/ops/users":                    false,
		"PATCH /api/ops/users/{userId}":          false,
		"PATCH /api/ops/users/{userId}/password": false,
	}
	for _, route := range routes {
		key := route.Key()
		if _, ok := want[key]; ok {
			want[key] = true
		}
		if strings.HasPrefix(route.Path, "/api/ops/customers") {
			t.Fatalf("legacy customer route is still published: %s", key)
		}
	}
	for route, found := range want {
		if !found {
			t.Errorf("missing global user route %s", route)
		}
	}
}
