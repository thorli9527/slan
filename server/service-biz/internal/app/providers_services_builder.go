package app

func newServices(deps UseCaseDependencies) Services {
	deviceServices := newDeviceServices(deps)
	networkServices := newNetworkServices(deps)
	opsServices := newOpsServices(deps)
	mqttServices := newMQTTServices(deps)
	return Services{
		Devices:   deviceServices,
		Network:   networkServices,
		Ops:       opsServices,
		Wire:      newWireServices(deps, opsServices.NodeRegistry),
		Messaging: mqttServices,
	}
}
