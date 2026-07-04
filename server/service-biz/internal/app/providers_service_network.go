package app

import servicepkg "github.com/slan/service-biz/internal/service"

func newNetworkServices(deps UseCaseDependencies) NetworkServices {
	repos := deps.networkRepositories()
	ids := deps.networkIDs()
	broadcaster := servicepkg.NewNetworkBroadcastPublisher(deps.mqttConfig())
	return NetworkServices{
		CoreAccess: servicepkg.NetworkCoreService{
			Users:        repos.Users,
			Devices:      repos.Devices,
			Networks:     repos.Networks,
			Ops:          repos.Ops,
			Broadcaster:  broadcaster,
			NewNetworkID: ids.NewNetworkID,
			Now:          deps.now(),
		},
		InviteManagement: servicepkg.NetworkInviteService{
			Users:       repos.Users,
			Devices:     repos.Devices,
			Networks:    repos.Networks,
			Broadcaster: broadcaster,
			NewInviteID: ids.NewInviteID,
			Now:         deps.now(),
		},
		DNSManagement: servicepkg.NetworkDNSService{
			Users:       repos.Users,
			Devices:     repos.Devices,
			Networks:    repos.Networks,
			Ops:         repos.Ops,
			Broadcaster: broadcaster,
			Now:         deps.now(),
		},
		AccessManagement: servicepkg.NetworkAccessService{
			Users:       repos.Users,
			Devices:     repos.Devices,
			Networks:    repos.Networks,
			Ops:         repos.Ops,
			Broadcaster: broadcaster,
			Now:         deps.now(),
		},
		RuntimeControl: servicepkg.NetworkRuntimeService{
			Devices:   repos.Devices,
			Networks:  repos.Networks,
			Ops:       repos.Ops,
			NewSessID: ids.NewSessionID,
		},
	}
}
