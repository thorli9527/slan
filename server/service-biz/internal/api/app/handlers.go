package app

import (
	serviceapi "github.com/slan/service-biz/internal/api"
	servicepkg "github.com/slan/service-biz/internal/service"
)

type RouteDependencies struct {
	DeviceCore       servicepkg.DeviceCoreUseCase
	DeviceCredential servicepkg.DeviceCredentialUseCase
	DeviceSession    servicepkg.DeviceSessionUseCase
	ClientMessages   servicepkg.ClientMessageUseCase
	NetworkCore      servicepkg.NetworkCoreUseCase
	NetworkRuntime   servicepkg.NetworkRuntimeUseCase
}

func Routes(deps RouteDependencies) []serviceapi.Route {
	appRoutes := serviceapi.WithRequiredPrefix(serviceapi.CombineRoutes(
		deviceRoutes(deps),
		networkRoutes(deps),
	), "/api/app")
	return serviceapi.CombineRoutes(DeviceCredentialHandler{
		Credentials: deps.DeviceCredential,
		Limiter:     newDeviceCredentialExchangeLimiter(),
	}.Routes(), appRoutes)
}

func deviceRoutes(deps RouteDependencies) []serviceapi.Route {
	runtimeLimiter := newDeviceRequestLimiter(deviceRuntimeReportLimit, deviceRuntimeReportIPLimit)
	return serviceapi.CombineRoutes(
		DeviceHandler{
			Devices: deps.DeviceCore, DeviceSessions: deps.DeviceSession, NetworkCore: deps.NetworkCore,
			RuntimeLimiter: runtimeLimiter,
		}.Routes(),
		DeviceSessionHandler{
			DeviceSessions:    deps.DeviceSession,
			NetworkRuntime:    deps.NetworkRuntime,
			NetworkConfigView: deps.NetworkCore,
			Limiter:           newDeviceRequestLimiter(deviceSessionRenewLimit, deviceSessionRenewIPLimit),
		}.Routes(),
		DeviceConfigHandler{Devices: deps.DeviceCore, DeviceSessions: deps.DeviceSession, NetworkCore: deps.NetworkCore}.Routes(),
		DeviceLogHandler{DeviceSessions: deps.DeviceSession}.Routes(),
		ClientMessageHandler{Messages: deps.ClientMessages, DeviceSessions: deps.DeviceSession}.Routes(),
	)
}

func networkRoutes(deps RouteDependencies) []serviceapi.Route {
	return serviceapi.CombineRoutes(
		NetworkSnapshotHandler{Snapshots: deps.NetworkCore, DeviceSessions: deps.DeviceSession}.Routes(),
		NetworkRuntimeHandler{
			NetworkRuntime:    deps.NetworkRuntime,
			DeviceSessions:    deps.DeviceSession,
			NetworkConfigView: deps.NetworkCore,
		}.Routes(),
	)
}
