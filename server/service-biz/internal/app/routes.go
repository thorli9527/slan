package app

import (
	"strings"
)

const (
	RouteSetAll  = "all"
	RouteSetApp  = "app"
	RouteSetWire = "wire"
	RouteSetMQTT = "mqtt"
	RouteSetOps  = "ops"
	RouteSetOpt  = "opt"
)

var routeSetAliases = map[string]string{
	"":           RouteSetAll,
	RouteSetAll:  RouteSetAll,
	RouteSetApp:  RouteSetApp,
	RouteSetWire: RouteSetWire,
	RouteSetMQTT: RouteSetMQTT,
	RouteSetOps:  RouteSetOps,
	RouteSetOpt:  RouteSetOps,
}

func normalizeRouteSet(routeSet string) string {
	return canonicalRouteSet(strings.TrimSpace(routeSet))
}

func canonicalRouteSet(routeSet string) string {
	if normalized, ok := routeSetAliases[routeSet]; ok {
		return normalized
	}
	return RouteSetAll
}
