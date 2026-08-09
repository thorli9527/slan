package app

import (
	serviceapi "github.com/slan/service-biz/internal/api"
	appapi "github.com/slan/service-biz/internal/api/app"
	opsapi "github.com/slan/service-biz/internal/api/ops"
)

func appRouteDependencies(useCases RouteUseCases) appapi.RouteDependencies {
	return appapi.RouteDependencies{
		DeviceCore:       useCases.App.Devices,
		DeviceCredential: useCases.App.DeviceCredentials,
		DeviceSession:    useCases.App.DeviceSessions,
		ClientMessages:   useCases.App.ClientMessages,
		NetworkCore:      useCases.App.NetworkCore,
		NetworkRuntime:   useCases.App.NetworkRuntime,
		ServerNodes:      useCases.App.ServerNodes,
	}
}

func opsRouteDependencies(useCases RouteUseCases) opsapi.RouteDependencies {
	return opsapi.RouteDependencies{
		AuthSessions:      useCases.Ops.SessionAuth,
		Operators:         useCases.Ops.OperatorDirectory,
		OperatorPasswords: useCases.Ops.OperatorSecurity,
		OverviewDashboard: useCases.Ops.DashboardOverview,
		OverviewAudit:     useCases.Ops.AuditOverview,
		ServerNode:        useCases.Ops.ServerNodes,
		Customer:          useCases.Ops.CustomerDirectory,
		ManagedDevice:     useCases.Ops.DeviceDirectory,
		DeviceCredential:  useCases.Ops.DeviceCredentials,
		Resources:         useCases.Ops.Resources,
		NetworkDNS:        useCases.Ops.NetworkDNS,
		NetworkAccess:     useCases.Ops.NetworkAccess,
	}
}

func appRoutes(useCases RouteUseCases) []serviceapi.Route {
	return appapi.Routes(appRouteDependencies(useCases))
}

func opsRoutes(useCases RouteUseCases) []serviceapi.Route {
	return opsapi.Routes(opsRouteDependencies(useCases))
}
