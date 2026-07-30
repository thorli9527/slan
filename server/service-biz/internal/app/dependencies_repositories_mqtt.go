package app

import "github.com/slan/service-biz/internal/repository"

type MQTTRepositories struct {
	Networks        repository.NetworkRepository
	EventDeliveries repository.NetworkEventDeliveryStore
}
