package app

import "github.com/slan/service-biz/internal/repository"

type OpsRepositories struct {
	Customers        repository.CustomerRepository
	Devices          repository.DeviceRepository
	DeviceInventory  repository.DeviceInventoryRepository
	Credentials      repository.DeviceCredentialRepository
	Networks         repository.NetworkRepository
	NetworkGroups    repository.NetworkDeviceGroupRepository
	Operators        repository.OperatorRepository
	OperatorSessions repository.OperatorSessionRepository
	Audit            repository.AuditRepository
	Nodes            repository.OpsNodeRepository
}
