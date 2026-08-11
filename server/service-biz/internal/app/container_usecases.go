package app

import servicepkg "github.com/slan/service-biz/internal/service"

type UseCases struct {
	Devices   DeviceUseCases
	Network   NetworkUseCases
	Ops       OpsUseCases
	Wire      WireUseCases
	Messaging MQTTUseCases
}

type DeviceUseCases struct {
	DeviceManagement servicepkg.DeviceCoreUseCase
	Credentials      servicepkg.DeviceCredentialUseCase
	SessionRuntime   servicepkg.DeviceSessionUseCase
	ClientMessages   servicepkg.ClientMessageUseCase
	DeviceOffline    servicepkg.DeviceOfflineUseCase
}

type NetworkUseCases struct {
	CoreAccess       servicepkg.NetworkCoreUseCase
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
	ServerNodes       servicepkg.OpsServerNodeUseCase
	CustomerDirectory servicepkg.OpsCustomerUseCase
	DeviceDirectory   servicepkg.OpsManagedDeviceUseCase
	DeviceCredentials servicepkg.DeviceCredentialUseCase
	Resources         servicepkg.OpsResourceUseCase
}

type WireUseCases struct {
	AdminControl servicepkg.WireUseCase
	NodeRegistry servicepkg.WireNodeUseCase
	PeerRegistry servicepkg.WirePeerUseCase
}

type MQTTUseCases struct {
	BrokerWebhook servicepkg.MQTTUseCase
}
