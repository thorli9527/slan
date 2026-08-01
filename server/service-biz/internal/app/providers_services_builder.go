package app

func newServices(deps UseCaseDependencies) Services {
	authServices := newAuthServices(deps)
	deviceServices := newDeviceServices(deps)
	networkServices := newNetworkServices(deps)
	opsServices := newOpsServices(deps)
	mqttServices := newMQTTServices(deps)
	return Services{
		Auth:      authServices,
		Devices:   deviceServices,
		Network:   networkServices,
		Ops:       opsServices,
		Wire:      newWireServices(deps, opsServices.NodeRegistry),
		Messaging: mqttServices,
	}
}
