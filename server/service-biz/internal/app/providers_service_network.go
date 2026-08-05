package app

import (
	"github.com/slan/service-biz/internal/geoip"
	servicepkg "github.com/slan/service-biz/internal/service"
)

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
			Devices:  repos.Devices,
			Networks: repos.Networks,
			Ops:      repos.Ops,
			LocateIP: func(ip string) (servicepkg.DeviceLocation, bool) {
				location, ok := geoip.DefaultLookup(ip)
				return servicepkg.DeviceLocation{
					CountryCode: location.CountryCode,
					CityCode:    location.CityCode,
				}, ok
			},
			NewSessID: ids.NewSessionID,
			Now:       deps.now(),
		},
	}
}
