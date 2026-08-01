package app

import (
	"strings"
	"testing"

	serviceapi "github.com/slan/service-biz/internal/api"
)

func TestNoRouteSetPublishesRetiredV2OrWebPrefix(t *testing.T) {
	catalog := newRouteCatalog(RouteUseCases{})
	for _, routes := range [][]routeListItem{
		asRouteList(catalog.publicRoutes()),
		asRouteList(catalog.allRoutes()),
		asRouteList(catalog.appBundleRoutes()),
	} {
		for _, route := range routes {
			if route.path == "/api/v2" || strings.HasPrefix(route.path, "/api/v2/") ||
				route.path == "/api/web" || strings.HasPrefix(route.path, "/api/web/") {
				t.Fatalf("retired API prefix is still published: %s %s", route.method, route.path)
			}
		}
	}
}

type routeListItem struct {
	method string
	path   string
}

func asRouteList(routes []serviceapi.Route) []routeListItem {
	items := make([]routeListItem, 0, len(routes))
	for _, route := range routes {
		items = append(items, routeListItem{method: route.Method, path: route.Path})
	}
	return items
}
