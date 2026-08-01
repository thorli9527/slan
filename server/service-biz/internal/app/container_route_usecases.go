package app

import servicepkg "github.com/slan/service-biz/internal/service"

type RouteUseCases struct {
	App           AppRouteUseCases
	Management    ManagementRouteUseCases
	Ops           OpsRouteUseCases
	WireControl   servicepkg.WireUseCase
	MessagingHook servicepkg.MQTTUseCase
}

type AppRouteUseCases struct {
	AuthRegistration    servicepkg.AuthUserRegistrationUseCase
	AuthSessions        servicepkg.AuthUserSessionUseCase
	ConsoleKeys         servicepkg.AuthConsoleKeyUseCase
	ConsoleLogin        servicepkg.AuthConsoleLoginUseCase
	DeviceLoginPrepare  servicepkg.AuthDeviceLoginPrepareUseCase
	DeviceLoginComplete servicepkg.AuthDeviceLoginCompleteUseCase
	Devices             servicepkg.DeviceCoreUseCase
	DeviceBootstrap     servicepkg.DeviceBootstrapUseCase
	DeviceSessions      servicepkg.DeviceSessionUseCase
	ClientMessages      servicepkg.ClientMessageUseCase
	NetworkCore         servicepkg.NetworkCoreUseCase
	NetworkInvite       servicepkg.NetworkInviteUseCase
	NetworkRuntime      servicepkg.NetworkRuntimeUseCase
}

type ManagementRouteUseCases struct {
	AuthSessions        servicepkg.AuthUserSessionUseCase
	UserTokens          servicepkg.UserTokenManagementUseCase
	UserAccounts        servicepkg.AuthUserAccountUseCase
	AuthAlias           servicepkg.AuthAliasUseCase
	ConsoleKeys         servicepkg.AuthConsoleKeyUseCase
	ConsoleLogin        servicepkg.AuthConsoleLoginUseCase
	DeviceLoginPrepare  servicepkg.AuthDeviceLoginPrepareUseCase
	DeviceLoginComplete servicepkg.AuthDeviceLoginCompleteUseCase
	Devices             servicepkg.DeviceCoreUseCase
	DeviceTokens        servicepkg.DeviceTokenManagementUseCase
	DeviceBootstrap     servicepkg.DeviceBootstrapUseCase
	DeviceGroups        servicepkg.DeviceGroupUseCase
	NetworkCore         servicepkg.NetworkCoreUseCase
	NetworkInvite       servicepkg.NetworkInviteUseCase
	NetworkDNS          servicepkg.NetworkDNSUseCase
	NetworkAccess       servicepkg.NetworkAccessUseCase
}

type OpsRouteUseCases struct {
	SessionAuth       servicepkg.OpsAuthSessionUseCase
	OperatorDirectory servicepkg.OpsOperatorUseCase
	OperatorSecurity  servicepkg.OpsOperatorPasswordUseCase
	DashboardOverview servicepkg.OpsDashboardUseCase
	AuditOverview     servicepkg.OpsAuditUseCase
	NodeRegistry      servicepkg.OpsNodeUseCase
	UserDirectory     servicepkg.OpsUserUseCase
	DeviceDirectory   servicepkg.OpsManagedDeviceUseCase
}

func newRouteUseCases(useCases UseCases) RouteUseCases {
	return RouteUseCases{
		App: AppRouteUseCases{
			AuthRegistration:    useCases.Auth.UserRegistration,
			AuthSessions:        useCases.Auth.UserSessions,
			ConsoleKeys:         useCases.Auth.ConsoleKeys,
			ConsoleLogin:        useCases.Auth.ConsoleLogin,
			DeviceLoginPrepare:  useCases.Auth.DeviceLoginPrepare,
			DeviceLoginComplete: useCases.Auth.DeviceLoginComplete,
			Devices:             useCases.Devices.DeviceManagement,
			DeviceBootstrap:     useCases.Devices.BootstrapAuth,
			DeviceSessions:      useCases.Devices.SessionRuntime,
			ClientMessages:      useCases.Devices.ClientMessages,
			NetworkCore:         useCases.Network.CoreAccess,
			NetworkInvite:       useCases.Network.InviteManagement,
			NetworkRuntime:      useCases.Network.RuntimeControl,
		},
		Management: ManagementRouteUseCases{
			AuthSessions:        useCases.Auth.UserSessions,
			UserTokens:          useCases.Auth.UserTokens,
			UserAccounts:        useCases.Auth.UserAccounts,
			AuthAlias:           useCases.Auth.Aliases,
			ConsoleKeys:         useCases.Auth.ConsoleKeys,
			ConsoleLogin:        useCases.Auth.ConsoleLogin,
			DeviceLoginPrepare:  useCases.Auth.DeviceLoginPrepare,
			DeviceLoginComplete: useCases.Auth.DeviceLoginComplete,
			Devices:             useCases.Devices.DeviceManagement,
			DeviceTokens:        useCases.Devices.TokenManagement,
			DeviceBootstrap:     useCases.Devices.BootstrapAuth,
			DeviceGroups:        useCases.Devices.GroupManagement,
			NetworkCore:         useCases.Network.CoreAccess,
			NetworkInvite:       useCases.Network.InviteManagement,
			NetworkDNS:          useCases.Network.DNSManagement,
			NetworkAccess:       useCases.Network.AccessManagement,
		},
		Ops: OpsRouteUseCases{
			SessionAuth:       useCases.Ops.SessionAuth,
			OperatorDirectory: useCases.Ops.OperatorDirectory,
			OperatorSecurity:  useCases.Ops.OperatorSecurity,
			DashboardOverview: useCases.Ops.DashboardOverview,
			AuditOverview:     useCases.Ops.AuditOverview,
			NodeRegistry:      useCases.Ops.NodeRegistry,
			UserDirectory:     useCases.Ops.UserDirectory,
			DeviceDirectory:   useCases.Ops.DeviceDirectory,
		},
		WireControl:   useCases.Wire.AdminControl,
		MessagingHook: useCases.Messaging.BrokerWebhook,
	}
}
