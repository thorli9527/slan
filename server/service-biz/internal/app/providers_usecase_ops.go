package app

func newOpsUseCasesFromServices(services OpsServices) OpsUseCases {
	return OpsUseCases{
		SessionAuth:       services.SessionAuth,
		OperatorDirectory: services.OperatorDirectory,
		OperatorSecurity:  services.OperatorSecurity,
		DashboardOverview: services.DashboardOverview,
		AuditOverview:     services.AuditOverview,
		ServerNodes:       services.ServerNodes,
		CustomerDirectory: services.CustomerDirectory,
		DeviceDirectory:   services.DeviceDirectory,
		DeviceCredentials: services.DeviceCredentials,
		Resources:         services.Resources,
	}
}
