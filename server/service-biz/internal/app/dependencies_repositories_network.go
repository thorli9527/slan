package app

import "github.com/slan/service-biz/internal/repository"

type NetworkRepositories struct {
	Devices         repository.DeviceRepository
	Networks        repository.NetworkRepository
	RuntimeNodes    repository.RuntimeNodeRepository
	EventDeliveries repository.NetworkEventDeliveryStore
}
