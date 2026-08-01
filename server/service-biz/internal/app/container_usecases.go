package app

import servicepkg "github.com/slan/service-biz/internal/service"

type UseCases struct {
	Auth      AuthUseCases
	Devices   DeviceUseCases
	Network   NetworkUseCases
	Ops       OpsUseCases
	Wire      WireUseCases
	Messaging MQTTUseCases
}

type AuthUseCases struct {
	UserRegistration    servicepkg.AuthUserRegistrationUseCase
	UserSessions        servicepkg.AuthUserSessionUseCase
	UserTokens          servicepkg.UserTokenManagementUseCase
	UserAccounts        servicepkg.AuthUserAccountUseCase
	Aliases             servicepkg.AuthAliasUseCase
	ConsoleKeys         servicepkg.AuthConsoleKeyUseCase
	ConsoleLogin        servicepkg.AuthConsoleLoginUseCase
	DeviceLoginPrepare  servicepkg.AuthDeviceLoginPrepareUseCase
	DeviceLoginComplete servicepkg.AuthDeviceLoginCompleteUseCase
}

type DeviceUseCases struct {
	DeviceManagement servicepkg.DeviceCoreUseCase
	TokenManagement  servicepkg.DeviceTokenManagementUseCase
	BootstrapAuth    servicepkg.DeviceBootstrapUseCase
	GroupManagement  servicepkg.DeviceGroupUseCase
	SessionRuntime   servicepkg.DeviceSessionUseCase
	ClientMessages   servicepkg.ClientMessageUseCase
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
	UserDirectory     servicepkg.OpsUserUseCase
	DeviceDirectory   servicepkg.OpsManagedDeviceUseCase
}

type WireUseCases struct {
	AdminControl servicepkg.WireUseCase
	NodeRegistry servicepkg.WireNodeUseCase
	PeerRegistry servicepkg.WirePeerUseCase
}

type MQTTUseCases struct {
	BrokerWebhook servicepkg.MQTTUseCase
}
