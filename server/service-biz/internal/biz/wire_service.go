package biz

// WireService 承载 wire/relay/DERP 内部接口使用的业务实现。
type WireService struct {
	store BusinessStore
}

func (s WireService) RelayNodes() []map[string]any {
	return wireRelayNodeViews(s.store.ListRelayNodes())
}

func (s WireService) DerpNodes() []map[string]any {
	return wireDerpNodeViews(s.store.ListRelayNodes())
}

func (s WireService) DerpMap() map[string]any {
	return derpMapFromRelayNodes(s.store.ListRelayNodes())
}

func (s WireService) PeerAuthz(peerID string) (map[string]any, error) {
	return s.store.WirePeerAuthz(peerID)
}

func (s WireService) PeerRuntimeConfig(peerID string) (map[string]any, error) {
	return s.store.WirePeerRuntimeConfig(peerID)
}

func (s WireService) NetworkTopology(networkID string) (map[string]any, error) {
	return s.store.WireNetworkTopology(networkID)
}

func (s WireService) UpsertRelayNode(req wireRelayNodeRequest) (map[string]any, error) {
	node, err := s.store.UpsertWireNode(req.relayNode())
	if err != nil {
		return nil, err
	}
	return wireRelayNodeView(node), nil
}

func (s WireService) RelayNodeHeartbeat(nodeID string, req wireNodeHeartbeatRequest) (map[string]any, error) {
	node, err := s.store.UpdateWireNodeHeartbeat(nodeID, "relay_udp", req)
	if err != nil {
		return nil, err
	}
	return wireRelayNodeView(node), nil
}

func (s WireService) RelayNodeStatus(nodeID string, req wireNodeStatusRequest) (map[string]any, error) {
	node, err := s.store.UpdateWireNodeStatus(nodeID, "relay_udp", req)
	if err != nil {
		return nil, err
	}
	return wireRelayNodeView(node), nil
}

func (s WireService) DeleteRelayNode(nodeID string) error {
	return s.store.DeleteWireNode(nodeID, "relay_udp")
}

func (s WireService) UpsertDerpNode(req wireDerpNodeRequest) (map[string]any, error) {
	node, err := s.store.UpsertWireNode(req.derpNode())
	if err != nil {
		return nil, err
	}
	return wireDerpNodeView(node), nil
}

func (s WireService) DerpNodeHeartbeat(nodeID string, req wireNodeHeartbeatRequest) (map[string]any, error) {
	node, err := s.store.UpdateWireNodeHeartbeat(nodeID, "derp_tcp_tls_443", req)
	if err != nil {
		return nil, err
	}
	return wireDerpNodeView(node), nil
}

func (s WireService) DerpNodeStatus(nodeID string, req wireNodeStatusRequest) (map[string]any, error) {
	node, err := s.store.UpdateWireNodeStatus(nodeID, "derp_tcp_tls_443", req)
	if err != nil {
		return nil, err
	}
	return wireDerpNodeView(node), nil
}

func (s WireService) DeleteDerpNode(nodeID string) error {
	return s.store.DeleteWireNode(nodeID, "derp_tcp_tls_443")
}
