package service

import "context"

type WireUseCase interface {
	WireNodeUseCase
	WirePeerUseCase
}

type WireService struct {
	Nodes WireNodeUseCase
	Peers WirePeerUseCase
}

func (s WireService) Authorize(ctx context.Context, token string) error {
	return s.Nodes.Authorize(ctx, token)
}

func (s WireService) ListRelayNodes(ctx context.Context) ([]WireNodeView, error) {
	return s.Nodes.ListRelayNodes(ctx)
}

func (s WireService) ListDerpNodes(ctx context.Context) ([]WireNodeView, error) {
	return s.Nodes.ListDerpNodes(ctx)
}

func (s WireService) UpsertRelayNode(ctx context.Context, input WireUpsertNodeInput) (WireNodeView, error) {
	return s.Nodes.UpsertRelayNode(ctx, input)
}

func (s WireService) UpsertDerpNode(ctx context.Context, input WireUpsertNodeInput) (WireNodeView, error) {
	return s.Nodes.UpsertDerpNode(ctx, input)
}

func (s WireService) HeartbeatRelayNode(ctx context.Context, input WireNodeStatusInput) (WireNodeView, error) {
	return s.Nodes.HeartbeatRelayNode(ctx, input)
}

func (s WireService) HeartbeatDerpNode(ctx context.Context, input WireNodeStatusInput) (WireNodeView, error) {
	return s.Nodes.HeartbeatDerpNode(ctx, input)
}

func (s WireService) UpdateRelayNodeStatus(ctx context.Context, input WireNodeStatusInput) (WireNodeView, error) {
	return s.Nodes.UpdateRelayNodeStatus(ctx, input)
}

func (s WireService) UpdateDerpNodeStatus(ctx context.Context, input WireNodeStatusInput) (WireNodeView, error) {
	return s.Nodes.UpdateDerpNodeStatus(ctx, input)
}

func (s WireService) DeleteRelayNode(ctx context.Context, input WireNodeDeleteInput) error {
	return s.Nodes.DeleteRelayNode(ctx, input)
}

func (s WireService) DeleteDerpNode(ctx context.Context, input WireNodeDeleteInput) error {
	return s.Nodes.DeleteDerpNode(ctx, input)
}

func (s WireService) DerpMap(ctx context.Context) (WireDerpMapView, error) {
	return s.Nodes.DerpMap(ctx)
}

func (s WireService) PeerAuthz(ctx context.Context, peerID string) (WirePeerAuthzView, error) {
	return s.Peers.PeerAuthz(ctx, peerID)
}

func (s WireService) PeerRuntimeConfig(ctx context.Context, peerID string) (WirePeerRuntimeConfigView, error) {
	return s.Peers.PeerRuntimeConfig(ctx, peerID)
}

func (s WireService) NetworkTopology(ctx context.Context, networkID string) (WireTopologyView, error) {
	return s.Peers.NetworkTopology(ctx, networkID)
}

func (s WireService) ReportPeerPathHealth(ctx context.Context, input WirePeerPathHealthInput) error {
	return s.Peers.ReportPeerPathHealth(ctx, input)
}
