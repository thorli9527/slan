package app

import "github.com/slan/service-biz/internal/repository"

type AuthRepositories struct {
	Users       repository.UserRepository
	Sessions    repository.UserSessionRepository
	UserAliases repository.UserAliasRepository
	Devices     repository.DeviceRepository
}
