package app

type Container struct {
	UseCases      UseCases
	RouteUseCases RouteUseCases
}

func NewDefaultContainer() Container {
	return NewContainerFromDependencies(newDefaultContainerDependencies())
}

func NewContainerFromDependencies(deps ContainerDependencies) Container {
	providers := newGormProviders(deps.Persistence)
	return newContainer(newUseCases(providers.Repositories, providers.IDs, deps.Runtime))
}

func NewContainer(repositories Repositories, ids IDGenerators, runtime Runtime) Container {
	return newContainer(newUseCases(repositories, ids, runtime))
}

func newContainer(useCases UseCases) Container {
	return Container{
		UseCases:      useCases,
		RouteUseCases: newRouteUseCases(useCases),
	}
}
