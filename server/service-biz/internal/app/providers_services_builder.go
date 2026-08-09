package app

import servicepkg "github.com/slan/service-biz/internal/service"

func newServices(deps UseCaseDependencies) Services {
	deviceServices := newDeviceServices(deps)
	networkServices := newNetworkServices(deps)
	opsServices := newOpsServices(deps)
	runtimeNodeRegistry := servicepkg.RuntimeNodeRegistryService{Nodes: deps.wireRepositories().Nodes}
	mqttServices := newMQTTServices(deps)
	return Services{
		Devices:   deviceServices,
		Network:   networkServices,
		Ops:       opsServices,
		Wire:      newWireServices(deps, runtimeNodeRegistry),
		Messaging: mqttServices,
	}
}
