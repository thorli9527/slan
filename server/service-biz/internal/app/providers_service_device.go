package app

import servicepkg "github.com/slan/service-biz/internal/service"

func newDeviceServices(deps UseCaseDependencies) DeviceServices {
	repos := deps.deviceRepositories()
	ids := deps.deviceIDs()
	eventPublisher := servicepkg.NewNetworkEventPublisher(deps.mqttConfig())
	return DeviceServices{
		DeviceManagement: servicepkg.NewDeviceCoreService(repos.Users, repos.Devices, repos.Networks, deps.mqttConfig(), ids.NewDeviceID, nil),
		BootstrapAuth: servicepkg.NewDeviceBootstrapService(
			repos.Users,
			repos.Devices,
			repos.Networks,
			deps.mqttConfig(),
			ids.NewSessionID,
			nil,
		),
		GroupManagement: servicepkg.DeviceGroupService{
			Users:           repos.Users,
			Devices:         repos.Devices,
			Networks:        repos.Networks,
			NetworkGroups:   repos.NetworkGroups,
			EventPublisher:  eventPublisher,
			DevicePublisher: servicepkg.NewDeviceControlPublisher(deps.mqttConfig()),
		},
		SessionRuntime: servicepkg.DeviceSessionService{
			Users:     repos.Users,
			Devices:   repos.Devices,
			Networks:  repos.Networks,
			MQTT:      deps.mqttConfig(),
			NewSessID: ids.NewSessionID,
		},
		ClientMessages: servicepkg.ClientMessageService{
			Devices:  repos.Devices,
			Networks: repos.Networks,
			MQTT:     deps.mqttConfig(),
		},
	}
}
