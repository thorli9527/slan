package app

func newOpsUseCasesFromServices(services OpsServices) OpsUseCases {
	return OpsUseCases{
		SessionAuth:       services.SessionAuth,
		OperatorDirectory: services.OperatorDirectory,
		OperatorSecurity:  services.OperatorSecurity,
		DashboardOverview: services.DashboardOverview,
		AuditOverview:     services.AuditOverview,
		NodeRegistry:      services.NodeRegistry,
		UserDirectory:     services.UserDirectory,
		DeviceDirectory:   services.DeviceDirectory,
	}
}
