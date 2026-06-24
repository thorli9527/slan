package app

import servicepkg "github.com/slan/service-biz/internal/service"

func newWireUseCasesFromServices(services WireServices) WireUseCases {
	combined := servicepkg.WireService{
		Nodes: services.NodeRegistry,
		Peers: services.PeerRegistry,
	}
	return WireUseCases{
		AdminControl: combined,
		NodeRegistry: services.NodeRegistry,
		PeerRegistry: services.PeerRegistry,
	}
}
