package app

import servicepkg "github.com/slan/service-biz/internal/service"

type RouteUseCases struct {
	App            AppRouteUseCases
	Web            WebRouteUseCases
	Ops            OpsRouteUseCases
	WireControl    servicepkg.WireUseCase
	MessagingHook  servicepkg.MQTTUseCase
	DownloadClient servicepkg.DownloadUseCase
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

type WebRouteUseCases struct {
	AuthRegistration    servicepkg.AuthUserRegistrationUseCase
	AuthSessions        servicepkg.AuthUserSessionUseCase
	UserTokens          servicepkg.UserTokenManagementUseCase
	UserAccounts        servicepkg.AuthUserAccountUseCase
	UserEntitlements    servicepkg.AuthUserEntitlementUseCase
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
	Downloads           servicepkg.DownloadUseCase
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
	DownloadCatalog   servicepkg.OpsCatalogDownloadUseCase
	PlanCatalog       servicepkg.OpsCatalogPlanUseCase
	ProductCatalog    servicepkg.OpsCatalogProductUseCase
	OrderCatalog      servicepkg.OpsCatalogOrderUseCase
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
		Web: WebRouteUseCases{
			AuthRegistration:    useCases.Auth.UserRegistration,
			AuthSessions:        useCases.Auth.UserSessions,
			UserTokens:          useCases.Auth.UserTokens,
			UserAccounts:        useCases.Auth.UserAccounts,
			UserEntitlements:    useCases.Auth.UserEntitlements,
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
			Downloads:           useCases.Downloads.ClientDelivery,
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
			DownloadCatalog:   useCases.Ops.DownloadCatalog,
			PlanCatalog:       useCases.Ops.PlanCatalog,
			ProductCatalog:    useCases.Ops.ProductCatalog,
			OrderCatalog:      useCases.Ops.OrderCatalog,
		},
		WireControl:    useCases.Wire.AdminControl,
		MessagingHook:  useCases.Messaging.BrokerWebhook,
		DownloadClient: useCases.Downloads.ClientDelivery,
	}
}
