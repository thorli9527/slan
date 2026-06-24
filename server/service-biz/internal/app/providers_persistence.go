package app

import "github.com/slan/service-biz/internal/repository"

func newDefaultContainerDependencies() ContainerDependencies {
	return ContainerDependencies{
		Persistence: Persistence{
			Gorm: repository.GormConfig{},
		},
		Runtime: Runtime{
			MQTTConfig: newMQTTConfig(),
		},
	}
}
