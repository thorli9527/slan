package app

func newDeviceUseCasesFromServices(deviceServices DeviceServices, authServices AuthServices) DeviceUseCases {
	return DeviceUseCases{
		DeviceManagement: deviceServices.DeviceManagement,
		TokenManagement:  authServices.TokenManagement,
		BootstrapAuth:    deviceServices.BootstrapAuth,
		GroupManagement:  deviceServices.GroupManagement,
		SessionRuntime:   deviceServices.SessionRuntime,
		ClientMessages:   deviceServices.ClientMessages,
	}
}
