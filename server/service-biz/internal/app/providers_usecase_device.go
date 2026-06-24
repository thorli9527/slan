package app

func newDeviceUseCasesFromServices(services DeviceServices) DeviceUseCases {
	return DeviceUseCases{
		DeviceManagement: services.DeviceManagement,
		BootstrapAuth:    services.BootstrapAuth,
		GroupManagement:  services.GroupManagement,
		SessionRuntime:   services.SessionRuntime,
	}
}
