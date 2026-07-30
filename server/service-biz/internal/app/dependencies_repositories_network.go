package app

import "github.com/slan/service-biz/internal/repository"

type NetworkRepositories struct {
	Users           repository.UserRepository
	Devices         repository.DeviceRepository
	Relations       repository.DeviceRelationRepository
	Networks        repository.NetworkRepository
	Ops             repository.OpsRepository
	EventDeliveries repository.NetworkEventDeliveryStore
}
