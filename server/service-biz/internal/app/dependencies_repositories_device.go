package app

import "github.com/slan/service-biz/internal/repository"

type DeviceRepositories struct {
	Users         repository.UserRepository
	Devices       repository.DeviceRepository
	Relations     repository.DeviceRelationRepository
	Networks      repository.NetworkRepository
	NetworkGroups repository.NetworkDeviceGroupRepository
}
