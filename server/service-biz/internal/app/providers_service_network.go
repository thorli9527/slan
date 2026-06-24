package app

import servicepkg "github.com/slan/service-biz/internal/service"

func newNetworkServices(deps UseCaseDependencies) NetworkServices {
	repos := deps.networkRepositories()
	ids := deps.networkIDs()
	return NetworkServices{
		CoreAccess: servicepkg.NetworkCoreService{
			Users:        repos.Users,
			Devices:      repos.Devices,
			Networks:     repos.Networks,
			Ops:          repos.Ops,
			NewNetworkID: ids.NewNetworkID,
		},
		InviteManagement: servicepkg.NetworkInviteService{
			Users:       repos.Users,
			Devices:     repos.Devices,
			Networks:    repos.Networks,
			NewInviteID: ids.NewInviteID,
		},
		DNSManagement: servicepkg.NetworkDNSService{
			Users:    repos.Users,
			Networks: repos.Networks,
		},
		AccessManagement: servicepkg.NetworkAccessService{
			Users:    repos.Users,
			Networks: repos.Networks,
		},
		RuntimeControl: servicepkg.NetworkRuntimeService{
			Devices:   repos.Devices,
			Networks:  repos.Networks,
			Ops:       repos.Ops,
			NewSessID: ids.NewSessionID,
		},
	}
}
