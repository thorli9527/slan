package service

import (
	"context"
	"time"

	"github.com/slan/service-biz/internal/pkg/wirekit"
	"github.com/slan/service-biz/internal/repository"
)

type WireServiceBase struct {
	Ops      OpsNodeUseCase
	Devices  repository.DeviceRepository
	Networks repository.NetworkRepository
	Catalog  repository.OpsRepository
	Now      func() time.Time
}

type WireNodeService struct{ WireServiceBase }
type WirePeerService struct{ WireServiceBase }

func (s WireNodeService) Authorize(_ context.Context, token string) error {
	expected := normalizedWireExpectedToken()
	if expected == "" || normalizeWireToken(token) != expected {
		return ErrUnauthorized
	}
	return nil
}

func (s WireNodeService) ListRelayNodes(ctx context.Context) ([]WireNodeView, error) {
	items, err := s.Catalog.ListRelayNodes(ctx)
	if err != nil {
		return nil, err
	}
	return relayWireNodeViews(items), nil
}

func (s WireNodeService) ListDerpNodes(ctx context.Context) ([]WireNodeView, error) {
	items, err := s.Catalog.ListRelayNodes(ctx)
	if err != nil {
		return nil, err
	}
	return derpWireNodeViews(items), nil
}

func (s WireNodeService) DerpMap(ctx context.Context) (WireDerpMapView, error) {
	items, err := s.Catalog.ListRelayNodes(ctx)
	if err != nil {
		return WireDerpMapView{}, err
	}
	return derpMapFromOpsNodes(items), nil
}

func (s WireNodeService) UpsertRelayNode(ctx context.Context, input WireUpsertNodeInput) (WireNodeView, error) {
	item, err := s.Ops.UpsertRelayNode(ctx, UpsertNodeInput{
		NodeID:    normalizeWireNodeID(input.NodeID),
		Name:      normalizeWireName(input.NodeID, input.Name, "Relay Node"),
		Region:    normalizeWireRegion(input.RegionID),
		Endpoint:  wirekit.HostPort(input.Host, firstPositive(input.UDPPort, input.AdminPort)),
		Transport: "relay_udp",
		Priority:  input.Priority,
		Status:    wireNodeStatus(input.Enabled, input.Healthy),
	})
	if err != nil {
		return WireNodeView{}, err
	}
	node, ok, err := s.Catalog.GetRelayNode(ctx, normalizeWireNodeID(item.NodeID))
	if err != nil {
		return WireNodeView{}, err
	}
	if !ok {
		return WireNodeView{}, ErrNotFound
	}
	return relayNodeView(relayNodeModel(node, "relay_udp")), nil
}

func (s WireNodeService) UpsertDerpNode(ctx context.Context, input WireUpsertNodeInput) (WireNodeView, error) {
	item, err := s.Ops.UpsertRelayNode(ctx, UpsertNodeInput{
		NodeID:    normalizeWireNodeID(input.NodeID),
		Name:      normalizeWireName(input.NodeID, input.Name, "DERP Node"),
		Region:    normalizeWireRegion(input.RegionID),
		Endpoint:  wirekit.HostPort(input.Host, input.Port),
		Transport: "derp_tcp_tls_443",
		Priority:  input.Priority,
		Status:    wireNodeStatus(input.Enabled, input.Healthy),
	})
	if err != nil {
		return WireNodeView{}, err
	}
	node, ok, err := s.Catalog.GetRelayNode(ctx, normalizeWireNodeID(item.NodeID))
	if err != nil {
		return WireNodeView{}, err
	}
	if !ok {
		return WireNodeView{}, ErrNotFound
	}
	return derpNodeView(relayNodeModel(node, "derp_tcp_tls_443")), nil
}

func (s WireNodeService) HeartbeatRelayNode(ctx context.Context, input WireNodeStatusInput) (WireNodeView, error) {
	return s.UpdateRelayNodeStatus(ctx, WireNodeStatusInput{
		RegionID:          input.RegionID,
		NodeID:            input.NodeID,
		Healthy:           input.Healthy,
		TicketKeyRotation: input.TicketKeyRotation,
	})
}

func (s WireNodeService) HeartbeatDerpNode(ctx context.Context, input WireNodeStatusInput) (WireNodeView, error) {
	return s.UpdateDerpNodeStatus(ctx, WireNodeStatusInput{
		RegionID:          input.RegionID,
		NodeID:            input.NodeID,
		Healthy:           input.Healthy,
		TicketKeyRotation: input.TicketKeyRotation,
	})
}

func (s WireNodeService) UpdateRelayNodeStatus(ctx context.Context, input WireNodeStatusInput) (WireNodeView, error) {
	item, err := s.requireRelayNodeRegion(ctx, input.RegionID, input.NodeID)
	if err != nil {
		return WireNodeView{}, err
	}
	if input.Enabled != nil {
		applyWireNodeHealth(&item, input.Enabled, nil)
	}
	applyWireNodeHealth(&item, nil, input.Healthy)
	applyWireTicketKeyStatus(&item, input.TicketKeyRotation)
	item.UpdatedAt = wirekit.NowUnix()
	if err := s.Catalog.SaveRelayNode(ctx, item); err != nil {
		return WireNodeView{}, err
	}
	return relayNodeView(item), nil
}

func (s WireNodeService) UpdateDerpNodeStatus(ctx context.Context, input WireNodeStatusInput) (WireNodeView, error) {
	item, err := s.requireDerpNodeRegion(ctx, input.RegionID, input.NodeID)
	if err != nil {
		return WireNodeView{}, err
	}
	if input.Enabled != nil {
		applyWireNodeHealth(&item, input.Enabled, nil)
	}
	applyWireNodeHealth(&item, nil, input.Healthy)
	applyWireTicketKeyStatus(&item, input.TicketKeyRotation)
	item.UpdatedAt = wirekit.NowUnix()
	if err := s.Catalog.SaveRelayNode(ctx, item); err != nil {
		return WireNodeView{}, err
	}
	return derpNodeView(item), nil
}

func (s WireNodeService) DeleteRelayNode(ctx context.Context, input WireNodeDeleteInput) error {
	if _, err := s.requireRelayNodeRegion(ctx, input.RegionID, input.NodeID); err != nil {
		return err
	}
	return s.Ops.DeleteRelayNode(ctx, input.NodeID)
}

func (s WireNodeService) DeleteDerpNode(ctx context.Context, input WireNodeDeleteInput) error {
	if _, err := s.requireDerpNodeRegion(ctx, input.RegionID, input.NodeID); err != nil {
		return err
	}
	return s.Ops.DeleteRelayNode(ctx, input.NodeID)
}

func (s WirePeerService) PeerAuthz(ctx context.Context, peerID string) (WirePeerAuthzView, error) {
	networkID, deviceID, err := s.lookupPeer(ctx, peerID)
	if err != nil {
		return WirePeerAuthzView{}, err
	}
	device, err := getManagedDevice(ctx, s.Devices, deviceID)
	if err != nil {
		return WirePeerAuthzView{}, err
	}
	return wirePeerAuthzView(peerID, networkID, device), nil
}

func (s WirePeerService) PeerRuntimeConfig(ctx context.Context, peerID string) (WirePeerRuntimeConfigView, error) {
	networkID, deviceID, err := s.lookupPeer(ctx, peerID)
	if err != nil {
		return WirePeerRuntimeConfigView{}, err
	}
	device, err := getManagedDevice(ctx, s.Devices, deviceID)
	if err != nil {
		return WirePeerRuntimeConfigView{}, err
	}
	endpoints, membership, _ := peerEndpoints(ctx, s.Networks, networkID, deviceID)
	return wirePeerRuntimeConfigView(peerID, networkID, device, endpoints, membership), nil
}

func (s WirePeerService) NetworkTopology(ctx context.Context, networkID string) (WireTopologyView, error) {
	networkID = normalizeWireNetworkID(networkID)
	if networkID == "" {
		return WireTopologyView{}, ErrInvalidArgument
	}
	items, err := s.Networks.ListNetworkDevices(ctx, networkID)
	if err != nil {
		return WireTopologyView{}, err
	}
	peers, err := wireTopologyPeers(ctx, s.Devices, networkID, items)
	if err != nil {
		return WireTopologyView{}, err
	}
	return WireTopologyView{NetworkID: networkID, Peers: peers}, nil
}

func (s WirePeerService) ReportPeerPathHealth(ctx context.Context, input WirePeerPathHealthInput) error {
	input = normalizeWirePeerPathHealthInput(input)
	if input.PeerID == "" || len(input.Probes) == 0 {
		return ErrInvalidArgument
	}
	networkID, deviceID, err := s.lookupPeer(ctx, input.PeerID)
	if err != nil {
		return err
	}
	items, err := s.Networks.ListNetworkDevices(ctx, networkID)
	if err != nil {
		return err
	}
	probe := preferredWirePathProbe(input.Probes)
	for _, item := range items {
		if item.DeviceID != deviceID || !networkMemberActive(item) {
			continue
		}
		updated := applyWirePeerPathHealth(item, probe, currentTime(s.Now))
		return s.Networks.SaveNetworkDevice(ctx, updated)
	}
	return ErrNotFound
}

func preferredWirePathProbe(items []WirePathProbeInput) WirePathProbeInput {
	if len(items) == 0 {
		return WirePathProbeInput{}
	}
	best := items[0]
	bestScore := wireProbeSortScore(best)
	for _, item := range items[1:] {
		if score := wireProbeSortScore(item); score < bestScore {
			best = item
			bestScore = score
		}
	}
	return best
}

func wireProbeSortScore(item WirePathProbeInput) int64 {
	score := int64(1000000)
	if item.Reachable {
		score = 0
	}
	score += int64(item.LossPPM)
	score += int64(item.RTTMs) * 1000
	return score
}
