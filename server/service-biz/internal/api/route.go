package api

import (
	"net/http"
	"strings"
)

type Route struct {
	Method  string
	Path    string
	Handler http.HandlerFunc
}

func (r Route) Key() string {
	return r.Method + " " + r.Path
}

func NewRoute(method, path string, handler http.HandlerFunc) Route {
	return Route{Method: method, Path: path, Handler: handler}
}

func CombineRoutes(routeGroups ...[]Route) []Route {
	var routes []Route
	seen := make(map[string]struct{})
	for _, group := range routeGroups {
		for _, item := range group {
			key := item.Key()
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			routes = append(routes, item)
		}
	}
	return routes
}

func WithAliasPrefix(routes []Route, prefix string) []Route {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return CombineRoutes(routes)
	}
	aliased := make([]Route, 0, len(routes)*2)
	for _, route := range routes {
		aliased = append(aliased, route)
		if prefixed, ok := aliasPrefixedRoute(route, prefix); ok {
			aliased = append(aliased, prefixed)
		}
	}
	return CombineRoutes(aliased)
}

func WithRequiredPrefix(routes []Route, prefix string) []Route {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return CombineRoutes(routes)
	}
	prefixed := make([]Route, 0, len(routes))
	for _, route := range routes {
		if route.Path == prefix || strings.HasPrefix(route.Path, prefix+"/") {
			prefixed = append(prefixed, route)
			continue
		}
		if strings.HasPrefix(route.Path, "/api/") {
			prefixed = append(prefixed, NewRoute(route.Method, prefix+strings.TrimPrefix(route.Path, "/api"), route.Handler))
			continue
		}
		prefixed = append(prefixed, route)
	}
	return CombineRoutes(prefixed)
}

func aliasPrefixedRoute(route Route, prefix string) (Route, bool) {
	if !strings.HasPrefix(route.Path, "/api/") {
		return Route{}, false
	}
	if strings.HasPrefix(route.Path, prefix+"/") || route.Path == prefix {
		return Route{}, false
	}
	return NewRoute(route.Method, prefix+strings.TrimPrefix(route.Path, "/api"), route.Handler), true
}

func RegisterRoutes(mux *http.ServeMux, routes []Route) {
	for _, item := range routes {
		mux.HandleFunc(item.Method+" "+item.Path, item.Handler)
	}
}
