package ops

import (
	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type RouteDependencies struct {
	AuthSessions      servicepkg.OpsAuthSessionUseCase
	Operators         servicepkg.OpsOperatorUseCase
	OperatorPasswords servicepkg.OpsOperatorPasswordUseCase
	OverviewDashboard servicepkg.OpsDashboardUseCase
	OverviewAudit     servicepkg.OpsAuditUseCase
	Node              servicepkg.OpsNodeUseCase
	User              servicepkg.OpsUserUseCase
	ManagedDevice     servicepkg.OpsManagedDeviceUseCase
	DeviceGroup       servicepkg.DeviceGroupUseCase
	NetworkCore       servicepkg.NetworkCoreUseCase
	NetworkInvite     servicepkg.NetworkInviteUseCase
	NetworkDNS        servicepkg.NetworkDNSUseCase
	NetworkAccess     servicepkg.NetworkAccessUseCase
}

func Routes(deps RouteDependencies) []serviceapi.Route {
	return withRequiredOperatorSession(serviceapi.CombineRoutes(
		authRoutes(deps),
		overviewRoutes(deps),
		managementRoutes(deps),
	), deps.AuthSessions)
}

func authRoutes(deps RouteDependencies) []serviceapi.Route {
	return serviceapi.CombineRoutes(
		AuthHandler{OpsAuthSessions: deps.AuthSessions, OpsOperatorPasswords: deps.OperatorPasswords}.Routes(),
		OperatorHandler{OpsOperators: deps.Operators, OpsOperatorPasswords: deps.OperatorPasswords}.Routes(),
	)
}

func overviewRoutes(deps RouteDependencies) []serviceapi.Route {
	return serviceapi.CombineRoutes(
		OverviewHandler{OpsDashboardReader: deps.OverviewDashboard, OpsAudit: deps.OverviewAudit}.Routes(),
	)
}

func managementRoutes(deps RouteDependencies) []serviceapi.Route {
	return serviceapi.CombineRoutes(
		NodeHandler{OpsNodes: deps.Node}.Routes(),
		UserHandler{OpsUsers: deps.User}.Routes(),
		DeviceHandler{OpsDevices: deps.ManagedDevice}.Routes(),
		ResourceHandler{Users: deps.User, DeviceGroups: deps.DeviceGroup, Networks: deps.NetworkCore, NetworkInvite: deps.NetworkInvite, DNS: deps.NetworkDNS, Access: deps.NetworkAccess, Audit: deps.OverviewAudit}.Routes(),
	)
}
