package app

import (
	"net/http"
	"testing"

	serviceapi "github.com/slan/service-biz/internal/api"
)

func TestRoutesForSetAppIncludesWireRoutes(t *testing.T) {
	catalog := routeCatalog{
		app: []serviceapi.Route{
			serviceapi.NewRoute(http.MethodPost, "/api/device-auth/token", nil),
		},
		wire: []serviceapi.Route{
			serviceapi.NewRoute(http.MethodGet, "/internal/wire/admin/relay-nodes", nil),
		},
		mqtt: []serviceapi.Route{
			serviceapi.NewRoute(http.MethodPost, "/mqtt/bifromq/auth", nil),
		},
	}

	routes := routesForSet(catalog, RouteSetApp)
	keys := make(map[string]struct{}, len(routes))
	for _, route := range routes {
		keys[route.Key()] = struct{}{}
	}

	expect := []string{
		http.MethodPost + " /api/device-auth/token",
		http.MethodGet + " /internal/wire/admin/relay-nodes",
		http.MethodPost + " /mqtt/bifromq/auth",
	}
	for _, key := range expect {
		if _, ok := keys[key]; !ok {
			t.Fatalf("missing route %s in app route set", key)
		}
	}
}
