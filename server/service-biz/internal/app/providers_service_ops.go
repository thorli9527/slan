package app

import servicepkg "github.com/slan/service-biz/internal/service"

func newOpsServices(deps UseCaseDependencies) OpsServices {
	repos := deps.opsRepositories()
	ids := deps.opsIDs()
	networkIDs := deps.networkIDs()
	deviceIDs := deps.deviceIDs()
	eventPublisher := servicepkg.NewNetworkEventPublisher(deps.mqttConfig(), deps.networkRepositories().EventDeliveries)
	return OpsServices{
		SessionAuth: servicepkg.OpsAuthSessionService{
			Operators:         repos.Operators,
			OperatorSessions:  repos.OperatorSessions,
			Audit:             repos.Audit,
			NewOperatorSessID: ids.NewSessionID,
		},
		OperatorDirectory: servicepkg.OpsOperatorService{
			Operators: repos.Operators, OperatorSessions: repos.OperatorSessions, Audit: repos.Audit,
		},
		OperatorSecurity: servicepkg.OpsOperatorPasswordService{
			Operators: repos.Operators, OperatorSessions: repos.OperatorSessions, Audit: repos.Audit,
		},
		DashboardOverview: servicepkg.OpsDashboardService{
			Customers: repos.Customers,
			Devices:   repos.Devices,
			Inventory: repos.DeviceInventory,
			Networks:  repos.Networks,
			Operators: repos.Operators,
		},
		AuditOverview: servicepkg.OpsAuditService{
			Audit: repos.Audit,
		},
		NodeRegistry: servicepkg.OpsNodeService{
			Nodes: repos.Nodes,
		},
		CustomerDirectory: servicepkg.OpsCustomerService{
			Customers:     repos.Customers,
			NewCustomerID: ids.NewCustomerID,
		},
		DeviceDirectory: servicepkg.OpsManagedDeviceService{
			Devices:        repos.Devices,
			Inventory:      repos.DeviceInventory,
			Networks:       repos.Networks,
			Audit:          repos.Audit,
			EventPublisher: eventPublisher,
			NewDeviceID:    deviceIDs.NewDeviceID,
		},
		DeviceCredentials: servicepkg.DeviceCredentialService{
			Devices: repos.Devices, Credentials: repos.Credentials, Audit: repos.Audit, Networks: repos.Networks,
			MQTT: deps.mqttConfig(), Pepper: deviceCredentialPepper(), PreviousPeppers: deviceCredentialPreviousPeppers(), NewSessID: ids.NewSessionID,
		},
		Resources: servicepkg.OpsResourceService{
			Devices: repos.Devices, Networks: repos.Networks, NetworkGroups: repos.NetworkGroups,
			Ops: repos.Nodes, EventPublisher: eventPublisher,
			NewNetworkID: networkIDs.NewNetworkID,
			GroupRuntime: servicepkg.DeviceGroupService{
				Devices: repos.Devices, Networks: repos.Networks, NetworkGroups: repos.NetworkGroups,
				EventPublisher:  eventPublisher,
				DevicePublisher: servicepkg.NewDeviceControlPublisher(deps.mqttConfig()),
			},
		},
	}
}
