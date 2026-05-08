package service

import "github.com/slan/server/server-biz/api/dto"

// Wire exposes internal, read-only business authorization views to server-wire.
type Wire interface {
	PeerAuthz(peerID string) (dto.WirePeerAuthzView, error)
	PeerRuntimeConfig(peerID string) (dto.WirePeerRuntimeConfigView, error)
	NetworkTopology(networkID string) (dto.WireNetworkTopologyView, error)
	DerpMap() (dto.WireDerpMapView, error)
	ListDerpNodes() ([]dto.WireDerpNodeRecord, error)
	UpsertDerpNode(req dto.UpsertWireDerpNodeRequest) (dto.WireDerpNodeRecord, error)
	UpdateDerpNodeHealth(regionID, nodeID string, req dto.WireNodeHeartbeatRequest) (dto.WireDerpNodeRecord, error)
	UpdateDerpNodeStatus(regionID, nodeID string, req dto.UpdateWireNodeStatusRequest) (dto.WireDerpNodeRecord, error)
	ListRelayNodes() ([]dto.WireRelayNodeRecord, error)
	UpsertRelayNode(req dto.UpsertWireRelayNodeRequest) (dto.WireRelayNodeRecord, error)
	UpdateRelayNodeHealth(regionID, nodeID string, req dto.WireNodeHeartbeatRequest) (dto.WireRelayNodeRecord, error)
	UpdateRelayNodeStatus(regionID, nodeID string, req dto.UpdateWireNodeStatusRequest) (dto.WireRelayNodeRecord, error)
}
