package app

import "github.com/slan/service-biz/internal/repository"

type OpsRepositories struct {
	Users            repository.UserRepository
	Devices          repository.DeviceRepository
	Networks         repository.NetworkRepository
	Operators        repository.OperatorRepository
	OperatorSessions repository.OperatorSessionRepository
	Audit            repository.AuditRepository
	Catalog          repository.OpsRepository
}
