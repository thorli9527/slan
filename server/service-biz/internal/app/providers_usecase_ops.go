package app

func newOpsUseCasesFromServices(services OpsServices) OpsUseCases {
	return OpsUseCases{
		SessionAuth:       services.SessionAuth,
		OperatorDirectory: services.OperatorDirectory,
		OperatorSecurity:  services.OperatorSecurity,
		DashboardOverview: services.DashboardOverview,
		AuditOverview:     services.AuditOverview,
		NodeRegistry:      services.NodeRegistry,
		CustomerDirectory: services.CustomerDirectory,
		DeviceDirectory:   services.DeviceDirectory,
		DownloadCatalog:   services.DownloadCatalog,
		PlanCatalog:       services.PlanCatalog,
		ProductCatalog:    services.ProductCatalog,
		OrderCatalog:      services.OrderCatalog,
	}
}
