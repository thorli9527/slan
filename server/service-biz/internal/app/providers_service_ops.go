package app

import servicepkg "github.com/slan/service-biz/internal/service"

func newOpsServices(deps UseCaseDependencies) OpsServices {
	repos := deps.opsRepositories()
	ids := deps.opsIDs()
	return OpsServices{
		SessionAuth: servicepkg.OpsAuthSessionService{
			Operators:         repos.Operators,
			OperatorSessions:  repos.OperatorSessions,
			NewOperatorSessID: ids.NewSessionID,
		},
		OperatorDirectory: servicepkg.OpsOperatorService{
			Operators: repos.Operators,
		},
		OperatorSecurity: servicepkg.OpsOperatorPasswordService{
			Operators: repos.Operators,
		},
		DashboardOverview: servicepkg.OpsDashboardService{
			Users:     repos.Users,
			Devices:   repos.Devices,
			Networks:  repos.Networks,
			Operators: repos.Operators,
		},
		AuditOverview: servicepkg.OpsAuditService{
			Audit: repos.Audit,
		},
		NodeRegistry: servicepkg.OpsNodeService{
			Catalog: repos.Catalog,
		},
		CustomerDirectory: servicepkg.OpsCustomerService{
			Users:   repos.Users,
			Devices: repos.Devices,
			Catalog: repos.Catalog,
		},
		DeviceDirectory: servicepkg.OpsManagedDeviceService{
			Users:    repos.Users,
			Devices:  repos.Devices,
			Networks: repos.Networks,
		},
		DownloadCatalog: servicepkg.OpsCatalogDownloadService{
			Catalog: repos.Catalog,
		},
		PlanCatalog: servicepkg.OpsCatalogPlanService{
			Catalog: repos.Catalog,
		},
		ProductCatalog: servicepkg.OpsCatalogProductService{
			Catalog: repos.Catalog,
		},
		OrderCatalog: servicepkg.OpsCatalogOrderService{
			Users:   repos.Users,
			Catalog: repos.Catalog,
		},
	}
}
