package app

import (
	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type RouteDependencies struct {
	AuthRegistration    servicepkg.AuthUserRegistrationUseCase
	AuthSessions        servicepkg.AuthUserSessionUseCase
	ConsoleKeys         servicepkg.AuthConsoleKeyUseCase
	ConsoleLogin        servicepkg.AuthConsoleLoginUseCase
	DeviceLoginPrepare  servicepkg.AuthDeviceLoginPrepareUseCase
	DeviceLoginComplete servicepkg.AuthDeviceLoginCompleteUseCase
	DeviceCore          servicepkg.DeviceCoreUseCase
	DeviceBootstrap     servicepkg.DeviceBootstrapUseCase
	DeviceSession       servicepkg.DeviceSessionUseCase
	ClientMessages      servicepkg.ClientMessageUseCase
	NetworkCore         servicepkg.NetworkCoreUseCase
	NetworkInvite       servicepkg.NetworkInviteUseCase
	NetworkRuntime      servicepkg.NetworkRuntimeUseCase
}

func Routes(deps RouteDependencies) []serviceapi.Route {
	return serviceapi.WithRequiredPrefix(serviceapi.CombineRoutes(
		authRoutes(deps),
		deviceRoutes(deps),
		networkRoutes(deps),
	), "/api/app")
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

func deviceRoutes(deps RouteDependencies) []serviceapi.Route {
	return serviceapi.CombineRoutes(
		DeviceHandler{Devices: deps.DeviceCore, DeviceSessions: deps.DeviceSession, NetworkCore: deps.NetworkCore}.Routes(),
		DeviceSessionHandler{
			DeviceBootstrap:   deps.DeviceBootstrap,
			DeviceSessions:    deps.DeviceSession,
			AuthSessions:      deps.AuthSessions,
			NetworkRuntime:    deps.NetworkRuntime,
			NetworkConfigView: deps.NetworkCore,
		}.Routes(),
		DeviceConfigHandler{Devices: deps.DeviceCore, DeviceSessions: deps.DeviceSession, NetworkCore: deps.NetworkCore}.Routes(),
		ClientMessageHandler{Messages: deps.ClientMessages, DeviceSessions: deps.DeviceSession}.Routes(),
	)
}

func networkRoutes(deps RouteDependencies) []serviceapi.Route {
	return serviceapi.CombineRoutes(
		NetworkInviteHandler{Invites: deps.NetworkInvite, AuthSessions: deps.AuthSessions}.Routes(),
		NetworkSnapshotHandler{Snapshots: deps.NetworkCore, DeviceSessions: deps.DeviceSession}.Routes(),
		NetworkRuntimeHandler{
			NetworkRuntime:    deps.NetworkRuntime,
			DeviceSessions:    deps.DeviceSession,
			NetworkConfigView: deps.NetworkCore,
		}.Routes(),
	)
}
