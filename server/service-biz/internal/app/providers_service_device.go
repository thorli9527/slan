package app

import servicepkg "github.com/slan/service-biz/internal/service"

func newDeviceServices(deps UseCaseDependencies) DeviceServices {
	repos := deps.deviceRepositories()
	ids := deps.deviceIDs()
	return DeviceServices{
		DeviceManagement: servicepkg.NewDeviceCoreService(repos.Devices, repos.Networks, deps.mqttConfig(), nil),
		Credentials: servicepkg.DeviceCredentialService{
			Devices: repos.Devices, Credentials: repos.Credentials, Audit: repos.Audit, Networks: repos.Networks,
			MQTT: deps.mqttConfig(), Pepper: deviceCredentialPepper(), PreviousPeppers: deviceCredentialPreviousPeppers(), NewSessID: ids.NewSessionID,
		},
		SessionRuntime: servicepkg.DeviceSessionService{
			Devices:     repos.Devices,
			Credentials: repos.Credentials,
			Audit:       repos.Audit,
			Networks:    repos.Networks,
			MQTT:        deps.mqttConfig(),
			NewSessID:   ids.NewSessionID,
		},
		ClientMessages: servicepkg.ClientMessageService{
			Devices:  repos.Devices,
			Networks: repos.Networks,
			MQTT:     deps.mqttConfig(),
		},
		DeviceOffline: servicepkg.DeviceOfflineService{
			Devices:     repos.Devices,
			Credentials: repos.Credentials,
			Audit:       repos.Audit,
			Publisher:   servicepkg.NewDeviceControlPublisher(deps.mqttConfig()),
		},
	}
}
