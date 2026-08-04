package app

import "github.com/slan/service-biz/internal/repository"

type DeviceRepositories struct {
	Devices       repository.DeviceRepository
	Credentials   repository.DeviceCredentialRepository
	Audit         repository.AuditRepository
	Networks      repository.NetworkRepository
	NetworkGroups repository.NetworkDeviceGroupRepository
}
