package app

import (
	"strings"
)

const (
	RouteSetAll      = "all"
	RouteSetApp      = "app"
	RouteSetWeb      = "web"
	RouteSetWire     = "wire"
	RouteSetMQTT     = "mqtt"
	RouteSetDownload = "download"
	RouteSetConsole  = "console"
	RouteSetOps      = "ops"
	RouteSetOpt      = "opt"
)

var routeSetAliases = map[string]string{
	"":               RouteSetAll,
	RouteSetAll:      RouteSetAll,
	RouteSetApp:      RouteSetApp,
	RouteSetWeb:      RouteSetWeb,
	RouteSetWire:     RouteSetWire,
	RouteSetMQTT:     RouteSetMQTT,
	RouteSetDownload: RouteSetDownload,
	RouteSetConsole:  RouteSetWeb,
	RouteSetOps:      RouteSetOps,
	RouteSetOpt:      RouteSetOps,
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
