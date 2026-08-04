package app

import servicepkg "github.com/slan/service-biz/internal/service"

type Services struct {
	Devices   DeviceServices
	Network   NetworkServices
	Ops       OpsServices
	Wire      WireServices
	Messaging MQTTServices
}

type DeviceServices struct {
	DeviceManagement servicepkg.DeviceCoreService
	Credentials      servicepkg.DeviceCredentialService
	SessionRuntime   servicepkg.DeviceSessionService
	ClientMessages   servicepkg.ClientMessageService
}

type NetworkServices struct {
	CoreAccess       servicepkg.NetworkCoreService
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
	CustomerDirectory servicepkg.OpsCustomerService
	DeviceDirectory   servicepkg.OpsManagedDeviceService
	DeviceCredentials servicepkg.DeviceCredentialService
	Resources         servicepkg.OpsResourceService
}

type WireServices struct {
	NodeRegistry servicepkg.WireNodeService
	PeerRegistry servicepkg.WirePeerService
}

type MQTTServices struct {
	BrokerWebhook servicepkg.MQTTWebhookService
}
