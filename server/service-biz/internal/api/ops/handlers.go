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
	Customer          servicepkg.OpsCustomerUseCase
	ManagedDevice     servicepkg.OpsManagedDeviceUseCase
	CatalogDownloads  servicepkg.OpsCatalogDownloadUseCase
	CatalogPlans      servicepkg.OpsCatalogPlanUseCase
	CatalogProducts   servicepkg.OpsCatalogProductUseCase
	CatalogOrders     servicepkg.OpsCatalogOrderUseCase
}

func Routes(deps RouteDependencies) []serviceapi.Route {
	return serviceapi.CombineRoutes(
		authRoutes(deps),
		overviewRoutes(deps),
		managementRoutes(deps),
		catalogRoutes(deps),
	)
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
		CustomerHandler{OpsCustomers: deps.Customer}.Routes(),
		DeviceHandler{OpsDevices: deps.ManagedDevice}.Routes(),
	)
}

func catalogRoutes(deps RouteDependencies) []serviceapi.Route {
	return serviceapi.CombineRoutes(
		ClientDownloadHandler{OpsCatalogDownloads: deps.CatalogDownloads}.Routes(),
		PlanHandler{OpsCatalogPlans: deps.CatalogPlans}.Routes(),
		ProductHandler{OpsCatalogProducts: deps.CatalogProducts}.Routes(),
		OrderHandler{OpsCatalogOrders: deps.CatalogOrders}.Routes(),
	)
}
