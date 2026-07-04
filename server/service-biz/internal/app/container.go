package app

type Container struct {
	Services      Services
	UseCases      UseCases
	RouteUseCases RouteUseCases
}

func NewDefaultContainer() Container {
	return NewContainerFromDependencies(newDefaultContainerDependencies())
}

func NewContainerFromDependencies(deps ContainerDependencies) Container {
	providers := newGormProviders(deps.Persistence)
	services := newServices(UseCaseDependencies{
		Repositories: providers.Repositories,
		IDs:          providers.IDs,
		Runtime:      deps.Runtime,
	})
	return newContainerWithServices(services, newUseCasesFromServices(services))
}

func NewContainer(repositories Repositories, ids IDGenerators, runtime Runtime) Container {
	services := newServices(UseCaseDependencies{
		Repositories: repositories,
		IDs:          ids,
		Runtime:      runtime,
	})
	return newContainerWithServices(services, newUseCasesFromServices(services))
}

func newContainerWithServices(services Services, useCases UseCases) Container {
	return Container{
		Services:      services,
		UseCases:      useCases,
		RouteUseCases: newRouteUseCases(useCases),
	}
}

func newContainer(useCases UseCases) Container {
	return newContainerWithServices(Services{}, useCases)
}
