package app

func newUseCases(repositories Repositories, ids IDGenerators, runtime Runtime) UseCases {
	deps := UseCaseDependencies{
		Repositories: repositories,
		IDs:          ids,
		Runtime:      runtime,
	}
	return newUseCasesFromServices(newServices(deps))
}

func newUseCasesFromServices(services Services) UseCases {
	return UseCases{
		Devices:   newDeviceUseCasesFromServices(services.Devices),
		Network:   newNetworkUseCasesFromServices(services.Network),
		Ops:       newOpsUseCasesFromServices(services.Ops),
		Wire:      newWireUseCasesFromServices(services.Wire),
		Messaging: newMQTTUseCasesFromServices(services.Messaging),
	}
}
