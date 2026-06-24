package app

import "github.com/slan/service-biz/internal/repository"

type NetworkRepositories struct {
	Users    repository.UserRepository
	Devices  repository.DeviceRepository
	Networks repository.NetworkRepository
	Ops      repository.OpsRepository
}
