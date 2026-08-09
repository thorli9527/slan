package app

import servicepkg "github.com/slan/service-biz/internal/service"

func newWireServices(deps UseCaseDependencies, registry servicepkg.RuntimeNodeRegistryService) WireServices {
	repos := deps.wireRepositories()
	base := servicepkg.WireServiceBase{
		Registry: registry,
		Devices:  repos.Devices,
		Networks: repos.Networks,
		Nodes:    repos.Nodes,
	}
	return WireServices{
		NodeRegistry: servicepkg.WireNodeService{WireServiceBase: base},
		PeerRegistry: servicepkg.WirePeerService{WireServiceBase: base},
	}
}
