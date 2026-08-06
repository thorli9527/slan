package app

import (
	"time"

	"github.com/slan/service-biz/internal/pkg/mqttkit"
	"github.com/slan/service-biz/internal/repository"
)

type Runtime struct {
	MQTTConfig       mqttkit.Config
	DeviceRuntime    repository.DeviceRuntimeRepository
	DeviceRuntimeTTL time.Duration
}

type Persistence struct {
	Gorm repository.GormConfig
}
