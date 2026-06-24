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
		Auth:      newAuthUseCasesFromServices(services.Auth),
		Devices:   newDeviceUseCasesFromServices(services.Devices),
		Downloads: newDownloadUseCasesFromServices(services.Downloads),
		Network:   newNetworkUseCasesFromServices(services.Network),
		Ops:       newOpsUseCasesFromServices(services.Ops),
		Wire:      newWireUseCasesFromServices(services.Wire),
		Messaging: newMQTTUseCasesFromServices(services.Messaging),
	}
}
