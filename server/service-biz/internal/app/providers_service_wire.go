package app

import servicepkg "github.com/slan/service-biz/internal/service"

func newWireServices(deps UseCaseDependencies, ops servicepkg.OpsNodeService) WireServices {
	repos := deps.wireRepositories()
	base := servicepkg.WireServiceBase{
		Ops:      ops,
		Devices:  repos.Devices,
		Networks: repos.Networks,
		Catalog:  repos.Catalog,
	}
	return WireServices{
		NodeRegistry: servicepkg.WireNodeService{WireServiceBase: base},
		PeerRegistry: servicepkg.WirePeerService{WireServiceBase: base},
	}
}
