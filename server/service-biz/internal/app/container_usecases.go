package app

import servicepkg "github.com/slan/service-biz/internal/service"

type UseCases struct {
	Auth      AuthUseCases
	Devices   DeviceUseCases
	Downloads DownloadUseCases
	Network   NetworkUseCases
	Ops       OpsUseCases
	Wire      WireUseCases
	Messaging MQTTUseCases
}

type AuthUseCases struct {
	UserRegistration    servicepkg.AuthUserRegistrationUseCase
	UserSessions        servicepkg.AuthUserSessionUseCase
	UserAccounts        servicepkg.AuthUserAccountUseCase
	UserEntitlements    servicepkg.AuthUserEntitlementUseCase
	Aliases             servicepkg.AuthAliasUseCase
	ConsoleKeys         servicepkg.AuthConsoleKeyUseCase
	ConsoleLogin        servicepkg.AuthConsoleLoginUseCase
	DeviceLoginPrepare  servicepkg.AuthDeviceLoginPrepareUseCase
	DeviceLoginComplete servicepkg.AuthDeviceLoginCompleteUseCase
}

type DeviceUseCases struct {
	DeviceManagement servicepkg.DeviceCoreUseCase
	BootstrapAuth    servicepkg.DeviceBootstrapUseCase
	GroupManagement  servicepkg.DeviceGroupUseCase
	SessionRuntime   servicepkg.DeviceSessionUseCase
}

type NetworkUseCases struct {
	CoreAccess       servicepkg.NetworkCoreUseCase
	InviteManagement servicepkg.NetworkInviteUseCase
	DNSManagement    servicepkg.NetworkDNSUseCase
	AccessManagement servicepkg.NetworkAccessUseCase
	RuntimeControl   servicepkg.NetworkRuntimeUseCase
}

type OpsUseCases struct {
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

type WireUseCases struct {
	AdminControl servicepkg.WireUseCase
	NodeRegistry servicepkg.WireNodeUseCase
	PeerRegistry servicepkg.WirePeerUseCase
}

type MQTTUseCases struct {
	BrokerWebhook servicepkg.MQTTUseCase
}

type DownloadUseCases struct {
	ClientDelivery servicepkg.DownloadUseCase
}
