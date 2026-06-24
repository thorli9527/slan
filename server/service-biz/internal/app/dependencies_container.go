package app

type UseCaseDependencies struct {
	Repositories Repositories
	IDs          IDGenerators
	Runtime      Runtime
}

type ContainerDependencies struct {
	Persistence Persistence
	Runtime     Runtime
}
