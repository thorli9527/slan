package app

import "github.com/slan/service-biz/internal/repository"

type WireRepositories struct {
	Devices  repository.DeviceRepository
	Networks repository.NetworkRepository
	Nodes    repository.OpsNodeRepository
}
