package app

import (
	serviceapi "github.com/slan/service-biz/internal/api"
	appapi "github.com/slan/service-biz/internal/api/app"
	opsapi "github.com/slan/service-biz/internal/api/ops"
	webapi "github.com/slan/service-biz/internal/api/web"
)

func appRouteDependencies(useCases RouteUseCases) appapi.RouteDependencies {
	return appapi.RouteDependencies{
		AuthRegistration:    useCases.App.AuthRegistration,
		AuthSessions:        useCases.App.AuthSessions,
		ConsoleKeys:         useCases.App.ConsoleKeys,
		ConsoleLogin:        useCases.App.ConsoleLogin,
		DeviceLoginPrepare:  useCases.App.DeviceLoginPrepare,
		DeviceLoginComplete: useCases.App.DeviceLoginComplete,
		DeviceCore:          useCases.App.Devices,
		DeviceBootstrap:     useCases.App.DeviceBootstrap,
		DeviceSession:       useCases.App.DeviceSessions,
		ClientMessages:      useCases.App.ClientMessages,
		NetworkCore:         useCases.App.NetworkCore,
		NetworkInvite:       useCases.App.NetworkInvite,
		NetworkRuntime:      useCases.App.NetworkRuntime,
	}
}

func webRouteDependencies(useCases RouteUseCases) webapi.RouteDependencies {
	return webapi.RouteDependencies{
		AuthRegistration:    useCases.Web.AuthRegistration,
		AuthSessions:        useCases.Web.AuthSessions,
		UserTokens:          useCases.Web.UserTokens,
		UserAccounts:        useCases.Web.UserAccounts,
		UserEntitlements:    useCases.Web.UserEntitlements,
		AuthAlias:           useCases.Web.AuthAlias,
		ConsoleKeys:         useCases.Web.ConsoleKeys,
		ConsoleLogin:        useCases.Web.ConsoleLogin,
		DeviceLoginPrepare:  useCases.Web.DeviceLoginPrepare,
		DeviceLoginComplete: useCases.Web.DeviceLoginComplete,
		DeviceCore:          useCases.Web.Devices,
		DeviceTokens:        useCases.Web.DeviceTokens,
		DeviceBootstrap:     useCases.Web.DeviceBootstrap,
		DeviceGroup:         useCases.Web.DeviceGroups,
		NetworkCore:         useCases.Web.NetworkCore,
		NetworkInvite:       useCases.Web.NetworkInvite,
		NetworkDNS:          useCases.Web.NetworkDNS,
		NetworkAccess:       useCases.Web.NetworkAccess,
		Downloads:           useCases.Web.Downloads,
	}
}

func opsRouteDependencies(useCases RouteUseCases) opsapi.RouteDependencies {
	return opsapi.RouteDependencies{
		AuthSessions:      useCases.Ops.SessionAuth,
		Operators:         useCases.Ops.OperatorDirectory,
		OperatorPasswords: useCases.Ops.OperatorSecurity,
		OverviewDashboard: useCases.Ops.DashboardOverview,
		OverviewAudit:     useCases.Ops.AuditOverview,
		Node:              useCases.Ops.NodeRegistry,
		Customer:          useCases.Ops.CustomerDirectory,
		ManagedDevice:     useCases.Ops.DeviceDirectory,
		CatalogDownloads:  useCases.Ops.DownloadCatalog,
		CatalogPlans:      useCases.Ops.PlanCatalog,
		CatalogProducts:   useCases.Ops.ProductCatalog,
		CatalogOrders:     useCases.Ops.OrderCatalog,
	}
}

func appRoutes(useCases RouteUseCases) []serviceapi.Route {
	return appapi.Routes(appRouteDependencies(useCases))
}

func webRoutes(useCases RouteUseCases) []serviceapi.Route {
	return webapi.Routes(webRouteDependencies(useCases))
}

func opsRoutes(useCases RouteUseCases) []serviceapi.Route {
	return opsapi.Routes(opsRouteDependencies(useCases))
}
