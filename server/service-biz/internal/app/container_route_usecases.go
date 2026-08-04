package app

import servicepkg "github.com/slan/service-biz/internal/service"

type RouteUseCases struct {
	App           AppRouteUseCases
	Ops           OpsRouteUseCases
	WireControl   servicepkg.WireUseCase
	MessagingHook servicepkg.MQTTUseCase
}

type AppRouteUseCases struct {
	Devices           servicepkg.DeviceCoreUseCase
	DeviceCredentials servicepkg.DeviceCredentialUseCase
	DeviceSessions    servicepkg.DeviceSessionUseCase
	ClientMessages    servicepkg.ClientMessageUseCase
	NetworkCore       servicepkg.NetworkCoreUseCase
	NetworkRuntime    servicepkg.NetworkRuntimeUseCase
}

type OpsRouteUseCases struct {
	SessionAuth       servicepkg.OpsAuthSessionUseCase
	OperatorDirectory servicepkg.OpsOperatorUseCase
	OperatorSecurity  servicepkg.OpsOperatorPasswordUseCase
	DashboardOverview servicepkg.OpsDashboardUseCase
	AuditOverview     servicepkg.OpsAuditUseCase
	NodeRegistry      servicepkg.OpsNodeUseCase
	CustomerDirectory servicepkg.OpsCustomerUseCase
	DeviceDirectory   servicepkg.OpsManagedDeviceUseCase
	DeviceCredentials servicepkg.DeviceCredentialUseCase
	Resources         servicepkg.OpsResourceUseCase
	NetworkDNS        servicepkg.NetworkDNSUseCase
	NetworkAccess     servicepkg.NetworkAccessUseCase
}

func newRouteUseCases(useCases UseCases) RouteUseCases {
	return RouteUseCases{
		App: AppRouteUseCases{
			Devices:           useCases.Devices.DeviceManagement,
			DeviceCredentials: useCases.Devices.Credentials,
			DeviceSessions:    useCases.Devices.SessionRuntime,
			ClientMessages:    useCases.Devices.ClientMessages,
			NetworkCore:       useCases.Network.CoreAccess,
			NetworkRuntime:    useCases.Network.RuntimeControl,
		},
		Ops: OpsRouteUseCases{
			SessionAuth:       useCases.Ops.SessionAuth,
			OperatorDirectory: useCases.Ops.OperatorDirectory,
			OperatorSecurity:  useCases.Ops.OperatorSecurity,
			DashboardOverview: useCases.Ops.DashboardOverview,
			AuditOverview:     useCases.Ops.AuditOverview,
			NodeRegistry:      useCases.Ops.NodeRegistry,
			CustomerDirectory: useCases.Ops.CustomerDirectory,
			DeviceDirectory:   useCases.Ops.DeviceDirectory,
			DeviceCredentials: useCases.Ops.DeviceCredentials,
			Resources:         useCases.Ops.Resources,
			NetworkDNS:        useCases.Network.DNSManagement,
			NetworkAccess:     useCases.Network.AccessManagement,
		},
		WireControl:   useCases.Wire.AdminControl,
		MessagingHook: useCases.Messaging.BrokerWebhook,
	}
}
