package app

import (
	serviceapi "github.com/slan/service-biz/internal/api"
	mqttapi "github.com/slan/service-biz/internal/api/mqtt"
	wireapi "github.com/slan/service-biz/internal/api/wire"
)

type routeCatalog struct {
	app  []serviceapi.Route
	ops  []serviceapi.Route
	wire []serviceapi.Route
	mqtt []serviceapi.Route
}

func newRouteCatalog(useCases RouteUseCases) routeCatalog {
	return routeCatalog{
		app:  appRoutes(useCases),
		ops:  opsRoutes(useCases),
		wire: wireapi.Routes(useCases.WireControl),
		mqtt: mqttapi.Routes(useCases.MessagingHook),
	}
}

func (c routeCatalog) publicRoutes() []serviceapi.Route {
	return c.app
}

func (c routeCatalog) allRoutes() []serviceapi.Route {
	return serviceapi.CombineRoutes(c.publicRoutes(), c.ops, c.mqtt, c.wire)
}

func (c routeCatalog) appBundleRoutes() []serviceapi.Route {
	return serviceapi.CombineRoutes(c.app, c.wire, c.mqtt)
}
