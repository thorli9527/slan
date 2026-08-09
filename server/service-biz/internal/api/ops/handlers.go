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
	ServerNode        servicepkg.OpsServerNodeUseCase
	Customer          servicepkg.OpsCustomerUseCase
	ManagedDevice     servicepkg.OpsManagedDeviceUseCase
	DeviceCredential  servicepkg.DeviceCredentialUseCase
	Resources         servicepkg.OpsResourceUseCase
	NetworkDNS        servicepkg.NetworkDNSUseCase
	NetworkAccess     servicepkg.NetworkAccessUseCase
}

func Routes(deps RouteDependencies) []serviceapi.Route {
	return serviceapi.CombineRoutes(
		authRoutes(deps),
		overviewRoutes(deps),
		managementRoutes(deps),
	)
}

func authRoutes(deps RouteDependencies) []serviceapi.Route {
	return serviceapi.CombineRoutes(
		AuthHandler{
			OpsAuthSessions: deps.AuthSessions, OpsOperatorPasswords: deps.OperatorPasswords,
			LoginLimiter: newOpsLoginLimiter(),
		}.Routes(),
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
		ServerNodeHandler{ServerNodes: deps.ServerNode}.Routes(),
		CustomerHandler{OpsCustomers: deps.Customer}.Routes(),
		DeviceHandler{OpsDevices: deps.ManagedDevice}.Routes(),
		DeviceCredentialHandler{Credentials: deps.DeviceCredential}.Routes(),
		ResourceHandler{Resources: deps.Resources}.Routes(),
		NetworkPolicyHandler{DNS: deps.NetworkDNS, Access: deps.NetworkAccess}.Routes(),
	)
}
