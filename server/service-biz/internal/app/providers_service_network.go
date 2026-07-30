package app

import servicepkg "github.com/slan/service-biz/internal/service"

func newNetworkServices(deps UseCaseDependencies) NetworkServices {
	repos := deps.networkRepositories()
	ids := deps.networkIDs()
	eventPublisher := servicepkg.NewNetworkEventPublisher(deps.mqttConfig(), repos.EventDeliveries)
	devicePublisher := servicepkg.NewDeviceControlPublisher(deps.mqttConfig())
	return NetworkServices{
		CoreAccess: servicepkg.NetworkCoreService{
			Users:              repos.Users,
			Devices:            repos.Devices,
			Networks:           repos.Networks,
			Ops:                repos.Ops,
			EventPublisher:     eventPublisher,
			VersionPushTracker: servicepkg.NewNetworkVersionPushTracker(),
			NewNetworkID:       ids.NewNetworkID,
			Now:                deps.now(),
		},
		InviteManagement: servicepkg.NetworkInviteService{
			Users:           repos.Users,
			Devices:         repos.Devices,
			Relations:       repos.Relations,
			Networks:        repos.Networks,
			EventPublisher:  eventPublisher,
			DevicePublisher: devicePublisher,
			NewInviteID:     ids.NewInviteID,
			Now:             deps.now(),
		},
		DNSManagement: servicepkg.NetworkDNSService{
			Users:          repos.Users,
			Devices:        repos.Devices,
			Networks:       repos.Networks,
			Ops:            repos.Ops,
			EventPublisher: eventPublisher,
			Now:            deps.now(),
		},
		AccessManagement: servicepkg.NetworkAccessService{
			Users:          repos.Users,
			Devices:        repos.Devices,
			Networks:       repos.Networks,
			Ops:            repos.Ops,
			EventPublisher: eventPublisher,
			Now:            deps.now(),
		},
		RuntimeControl: servicepkg.NetworkRuntimeService{
			Devices:   repos.Devices,
			Networks:  repos.Networks,
			Ops:       repos.Ops,
			NewSessID: ids.NewSessionID,
		},
	}
}
