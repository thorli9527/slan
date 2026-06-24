package web

import (
	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type RouteDependencies struct {
	AuthRegistration    servicepkg.AuthUserRegistrationUseCase
	AuthSessions        servicepkg.AuthUserSessionUseCase
	UserAccounts        servicepkg.AuthUserAccountUseCase
	UserEntitlements    servicepkg.AuthUserEntitlementUseCase
	AuthAlias           servicepkg.AuthAliasUseCase
	ConsoleKeys         servicepkg.AuthConsoleKeyUseCase
	ConsoleLogin        servicepkg.AuthConsoleLoginUseCase
	DeviceLoginPrepare  servicepkg.AuthDeviceLoginPrepareUseCase
	DeviceLoginComplete servicepkg.AuthDeviceLoginCompleteUseCase
	DeviceCore          servicepkg.DeviceCoreUseCase
	DeviceBootstrap     servicepkg.DeviceBootstrapUseCase
	DeviceGroup         servicepkg.DeviceGroupUseCase
	NetworkCore         servicepkg.NetworkCoreUseCase
	NetworkInvite       servicepkg.NetworkInviteUseCase
	NetworkDNS          servicepkg.NetworkDNSUseCase
	NetworkAccess       servicepkg.NetworkAccessUseCase
	Downloads           servicepkg.DownloadUseCase
}

func Routes(deps RouteDependencies) []serviceapi.Route {
	return serviceapi.WithAliasPrefix(serviceapi.CombineRoutes(
		authRoutes(deps),
		userRoutes(deps),
		deviceRoutes(deps),
		networkRoutes(deps),
		downloadRoutes(deps),
	), "/api/web")
}

func authRoutes(deps RouteDependencies) []serviceapi.Route {
	return serviceapi.CombineRoutes(
		AuthHandler{
			AuthRegistration: deps.AuthRegistration,
			AuthSessions:     deps.AuthSessions,
			NetworkCore:      deps.NetworkCore,
		}.Routes(),
		AuthConsoleHandler{ConsoleKeys: deps.ConsoleKeys}.Routes(),
		AuthConsoleLoginHandler{ConsoleLoginUseCase: deps.ConsoleLogin}.Routes(),
		AuthDeviceLoginHandler{
			DeviceLoginPrepare:  deps.DeviceLoginPrepare,
			DeviceLoginComplete: deps.DeviceLoginComplete,
		}.Routes(),
	)
}

func userRoutes(deps RouteDependencies) []serviceapi.Route {
	return serviceapi.CombineRoutes(
		UserHandler{
			UserAccounts:     deps.UserAccounts,
			UserEntitlements: deps.UserEntitlements,
		}.Routes(),
		UserAliasHandler{AuthAlias: deps.AuthAlias}.Routes(),
	)
}

func deviceRoutes(deps RouteDependencies) []serviceapi.Route {
	return serviceapi.CombineRoutes(
		DeviceHandler{Devices: deps.DeviceCore}.Routes(),
		DeviceBootstrapHandler{DeviceBootstrap: deps.DeviceBootstrap, NetworkCore: deps.NetworkCore}.Routes(),
		DeviceGroupHandler{DeviceGroups: deps.DeviceGroup}.Routes(),
	)
}

func networkRoutes(deps RouteDependencies) []serviceapi.Route {
	return serviceapi.CombineRoutes(
		NetworkCoreHandler{NetworkCore: deps.NetworkCore}.Routes(),
		NetworkInviteHandler{NetworkInvite: deps.NetworkInvite}.Routes(),
		NetworkMembershipHandler{NetworkInvite: deps.NetworkInvite}.Routes(),
		NetworkDNSHandler{NetworkDNS: deps.NetworkDNS}.Routes(),
		NetworkMappingHandler{NetworkAccess: deps.NetworkAccess}.Routes(),
		NetworkSecurityHandler{NetworkAccess: deps.NetworkAccess}.Routes(),
	)
}

func downloadRoutes(deps RouteDependencies) []serviceapi.Route {
	return serviceapi.CombineRoutes(
		ClientDownloadHandler{Downloads: deps.Downloads}.Routes(),
	)
}
