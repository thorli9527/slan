package app

import "github.com/slan/service-biz/internal/repository"

type MQTTRepositories struct {
	Devices         repository.DeviceRepository
	Networks        repository.NetworkRepository
	Credentials     repository.DeviceCredentialRepository
	EventDeliveries repository.NetworkEventDeliveryStore
}
