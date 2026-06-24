package app

import (
	"github.com/slan/service-biz/internal/pkg/mqttkit"
	"github.com/slan/service-biz/internal/repository"
)

type Runtime struct {
	MQTTConfig mqttkit.Config
}

type Persistence struct {
	Gorm repository.GormConfig
}
