package app

import serviceapi "github.com/slan/service-biz/internal/api"

func (s *Server) routesFor(routeSet string) []serviceapi.Route {
	return routesForSet(newRouteCatalog(s.container.RouteUseCases), normalizeRouteSet(routeSet))
}

func routesForSet(c routeCatalog, routeSet string) []serviceapi.Route {
	switch routeSet {
	case RouteSetApp:
		return c.appBundleRoutes()
	case RouteSetWeb:
		return c.webBundleRoutes()
	case RouteSetOps:
		return c.ops
	case RouteSetWire:
		return c.wire
	case RouteSetMQTT:
		return c.mqtt
	case RouteSetDownload:
		return c.download
	default:
		return c.allRoutes()
	}
}
