package app

import (
	serviceapi "github.com/slan/service-biz/internal/api"
	downloadapi "github.com/slan/service-biz/internal/api/download"
	mqttapi "github.com/slan/service-biz/internal/api/mqtt"
	wireapi "github.com/slan/service-biz/internal/api/wire"
)

type routeCatalog struct {
	app      []serviceapi.Route
	web      []serviceapi.Route
	ops      []serviceapi.Route
	wire     []serviceapi.Route
	mqtt     []serviceapi.Route
	download []serviceapi.Route
}

func newRouteCatalog(useCases RouteUseCases) routeCatalog {
	return routeCatalog{
		app:      appRoutes(useCases),
		web:      webRoutes(useCases),
		ops:      opsRoutes(useCases),
		wire:     wireapi.Routes(useCases.WireControl),
		mqtt:     mqttapi.Routes(useCases.MessagingHook),
		download: downloadapi.Routes(useCases.DownloadClient),
	}
}

func (c routeCatalog) publicRoutes() []serviceapi.Route {
	return serviceapi.CombineRoutes(c.web, c.app, c.download)
}

func (c routeCatalog) allRoutes() []serviceapi.Route {
	return serviceapi.CombineRoutes(c.publicRoutes(), c.ops, c.mqtt, c.wire)
}

func (c routeCatalog) appBundleRoutes() []serviceapi.Route {
	return serviceapi.CombineRoutes(c.app, c.wire, c.mqtt, c.download)
}

func (c routeCatalog) webBundleRoutes() []serviceapi.Route {
	return serviceapi.CombineRoutes(c.web, c.download)
}
