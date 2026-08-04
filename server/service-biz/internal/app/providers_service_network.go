package app

import servicepkg "github.com/slan/service-biz/internal/service"

func newNetworkServices(deps UseCaseDependencies) NetworkServices {
	repos := deps.networkRepositories()
	ids := deps.networkIDs()
	eventPublisher := servicepkg.NewNetworkEventPublisher(deps.mqttConfig(), repos.EventDeliveries)
	return NetworkServices{
		CoreAccess: servicepkg.NetworkCoreService{
			Devices:            repos.Devices,
			Networks:           repos.Networks,
			Ops:                repos.Ops,
			EventPublisher:     eventPublisher,
			VersionPushTracker: servicepkg.NewNetworkVersionPushTracker(),
			NewNetworkID:       ids.NewNetworkID,
			Now:                deps.now(),
		},
		DNSManagement: servicepkg.NetworkDNSService{
			Devices:        repos.Devices,
			Networks:       repos.Networks,
			Ops:            repos.Ops,
			EventPublisher: eventPublisher,
			Now:            deps.now(),
		},
		AccessManagement: servicepkg.NetworkAccessService{
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
