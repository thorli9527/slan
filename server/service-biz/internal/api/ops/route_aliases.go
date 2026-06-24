package ops

import (
	"strings"

	serviceapi "github.com/slan/service-biz/internal/api"
)

func withOptAliases(routes []serviceapi.Route) []serviceapi.Route {
	result := make([]serviceapi.Route, 0, len(routes)*2)
	for _, route := range routes {
		result = append(result, route)
		if strings.HasPrefix(route.Path, "/api/ops/") {
			result = append(result, serviceapi.NewRoute(
				route.Method,
				"/api/opt/"+strings.TrimPrefix(route.Path, "/api/ops/"),
				route.Handler,
			))
		}
	}
	return result
}
