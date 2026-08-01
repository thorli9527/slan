package app

import (
	serviceapi "github.com/slan/service-biz/internal/api"
	appapi "github.com/slan/service-biz/internal/api/app"
	opsapi "github.com/slan/service-biz/internal/api/ops"
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

func opsRouteDependencies(useCases RouteUseCases) opsapi.RouteDependencies {
	return opsapi.RouteDependencies{
		AuthSessions:      useCases.Ops.SessionAuth,
		Operators:         useCases.Ops.OperatorDirectory,
		OperatorPasswords: useCases.Ops.OperatorSecurity,
		OverviewDashboard: useCases.Ops.DashboardOverview,
		OverviewAudit:     useCases.Ops.AuditOverview,
		Node:              useCases.Ops.NodeRegistry,
		User:              useCases.Ops.UserDirectory,
		ManagedDevice:     useCases.Ops.DeviceDirectory,
		DeviceGroup:       useCases.Management.DeviceGroups,
		NetworkCore:       useCases.Management.NetworkCore,
		NetworkInvite:     useCases.Management.NetworkInvite,
		NetworkDNS:        useCases.Management.NetworkDNS,
		NetworkAccess:     useCases.Management.NetworkAccess,
	}
}

func appRoutes(useCases RouteUseCases) []serviceapi.Route {
	return appapi.Routes(appRouteDependencies(useCases))
}

func opsRoutes(useCases RouteUseCases) []serviceapi.Route {
	return opsapi.Routes(opsRouteDependencies(useCases))
}
