package app

func newDeviceUseCasesFromServices(deviceServices DeviceServices) DeviceUseCases {
	return DeviceUseCases{
		DeviceManagement: deviceServices.DeviceManagement,
		Credentials:      deviceServices.Credentials,
		SessionRuntime:   deviceServices.SessionRuntime,
		ClientMessages:   deviceServices.ClientMessages,
	}
}
