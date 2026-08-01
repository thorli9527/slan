package app

import servicepkg "github.com/slan/service-biz/internal/service"

type Services struct {
	Auth      AuthServices
	Devices   DeviceServices
	Network   NetworkServices
	Ops       OpsServices
	Wire      WireServices
	Messaging MQTTServices
}

type AuthServices struct {
	UserAuth        servicepkg.AuthUserService
	TokenManagement servicepkg.TokenManagementService
	Aliases         servicepkg.AuthAliasService
	ConsoleAuth     servicepkg.AuthConsoleService
	DeviceLoginAuth servicepkg.AuthDeviceLoginService
}

type DeviceServices struct {
	DeviceManagement servicepkg.DeviceCoreService
	BootstrapAuth    servicepkg.DeviceBootstrapService
	GroupManagement  servicepkg.DeviceGroupService
	SessionRuntime   servicepkg.DeviceSessionService
	ClientMessages   servicepkg.ClientMessageService
}

type NetworkServices struct {
	CoreAccess       servicepkg.NetworkCoreService
	InviteManagement servicepkg.NetworkInviteService
	DNSManagement    servicepkg.NetworkDNSService
	AccessManagement servicepkg.NetworkAccessService
	RuntimeControl   servicepkg.NetworkRuntimeService
}

type OpsServices struct {
	SessionAuth       servicepkg.OpsAuthSessionService
	OperatorDirectory servicepkg.OpsOperatorService
	OperatorSecurity  servicepkg.OpsOperatorPasswordService
	DashboardOverview servicepkg.OpsDashboardService
	AuditOverview     servicepkg.OpsAuditService
	NodeRegistry      servicepkg.OpsNodeService
	UserDirectory     servicepkg.OpsUserService
	DeviceDirectory   servicepkg.OpsManagedDeviceService
}

type WireServices struct {
	NodeRegistry servicepkg.WireNodeService
	PeerRegistry servicepkg.WirePeerService
}

type MQTTServices struct {
	BrokerWebhook servicepkg.MQTTWebhookService
}
