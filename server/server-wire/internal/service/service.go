package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/slan/server/server-wire/internal/model"
	"github.com/slan/server/server-wire/internal/planner"
	"github.com/slan/server/server-wire/internal/store"
)

// Service 聚合 server-wire 的业务逻辑：peer 注册、路径健康更新、路径规划和票据签发。
type Service struct {
	store store.Store
	biz   BizAuthorizer
}

var (
	ErrNoHealthyRelayNodes = errors.New("relay has no schedulable enabled healthy nodes")
	ErrNoHealthyDerpNodes  = errors.New("derp has no schedulable enabled healthy nodes")
)

// BizAuthorizer 定义 server-wire 向 service-biz 查询授权、拓扑和节点调度数据的边界。
type BizAuthorizer interface {
	// Enabled 表示该授权源是否启用。
	Enabled() bool
	// PeerAuthz 查询 peer 是否允许接入及其网络身份。
	PeerAuthz(ctx context.Context, peerID string) (model.PeerAuthzView, error)
	// PeerRuntimeConfig 查询 peer 的运行配置。
	PeerRuntimeConfig(ctx context.Context, peerID string) (model.PeerRuntimeConfigView, error)
	// NetworkTopology 查询虚拟网络拓扑。
	NetworkTopology(ctx context.Context, networkID string) (model.NetworkTopologyView, error)
	// DerpMap 查询 DERP 节点地图。
	DerpMap(ctx context.Context) (model.DerpMap, error)
	// RelayNodes 查询可调度 UDP relay 节点。
	RelayNodes(ctx context.Context) ([]model.RelayNode, error)
}

// New 使用指定 Store 创建不连接 biz 的 server-wire 服务。
func New(st store.Store) *Service {
	return &Service{store: st}
}

// NewWithBiz 使用指定 Store 和 biz 授权源创建 server-wire 服务。
func NewWithBiz(st store.Store, biz BizAuthorizer) *Service {
	return &Service{store: st, biz: biz}
}

// RegisterPeer 校验 biz 授权后注册或刷新 peer 能力。
func (s *Service) RegisterPeer(req model.RegisterPeerRequest) (model.RegisterPeerResponse, error) {
	if err := s.applyBizAuthorization(context.Background(), &req.Peer); err != nil {
		return model.RegisterPeerResponse{}, err
	}
	peer, err := s.store.RegisterPeer(req.Peer)
	if err != nil {
		return model.RegisterPeerResponse{}, err
	}
	return model.RegisterPeerResponse{Peer: peer}, nil
}

func (s *Service) applyBizAuthorization(ctx context.Context, peer *model.PeerRegistration) error {
	if s.biz == nil || !s.biz.Enabled() {
		return nil
	}
	peerID := strings.TrimSpace(peer.PeerID)
	if peerID == "" {
		return fmt.Errorf("peerId is required")
	}
	authz, err := s.biz.PeerAuthz(ctx, peerID)
	if err != nil {
		return fmt.Errorf("biz authz denied: %w", err)
	}
	if !authz.Enabled {
		return fmt.Errorf("peer disabled by biz authz")
	}
	if authz.PeerID != "" && authz.PeerID != peer.PeerID {
		return fmt.Errorf("biz authz peer mismatch")
	}
	if authz.NetworkID != "" {
		peer.NetworkID = authz.NetworkID
	}
	if authz.NodeID != "" {
		peer.NodeID = authz.NodeID
	}
	if len(authz.VirtualIPs) > 0 {
		peer.VirtualIPs = append([]string(nil), authz.VirtualIPs...)
	}
	if len(authz.AllowedIPs) > 0 {
		peer.AllowedIPs = append([]string(nil), authz.AllowedIPs...)
	}
	return nil
}

func (s *Service) UpdateEndpoints(req model.UpdateEndpointsRequest) (model.PeerRecord, error) {
	if _, err := s.refreshPeerAuthorization(req.PeerID); err != nil {
		return model.PeerRecord{}, err
	}
	return s.store.UpdateEndpoints(req.PeerID, req.Endpoints)
}

func (s *Service) ReportPathHealth(req model.ReportPathHealthRequest) (model.PeerRecord, error) {
	if _, err := s.refreshPeerAuthorization(req.PeerID); err != nil {
		return model.PeerRecord{}, err
	}
	return s.store.UpdatePathHealth(req.PeerID, req.Probes)
}

func (s *Service) ReportDerpHealth(req model.ReportDerpHealthRequest) (model.PeerRecord, error) {
	if _, err := s.refreshPeerAuthorization(req.PeerID); err != nil {
		return model.PeerRecord{}, err
	}
	return s.store.UpdateDerpHealth(req.PeerID, req.Samples)
}

func (s *Service) UpdateActivePath(req model.UpdateActivePathRequest) (model.PeerRecord, error) {
	if _, err := s.refreshPeerAuthorization(req.PeerID); err != nil {
		return model.PeerRecord{}, err
	}
	return s.store.UpdateActivePath(req.PeerID, req.Path)
}

func (s *Service) IssueRelayTicket(req model.IssueRelayTicketRequest) (model.IssueRelayTicketResponse, error) {
	if _, err := s.refreshPeerAuthorization(req.PeerID); err != nil {
		return model.IssueRelayTicketResponse{}, err
	}
	relay, err := s.primaryRelayNode()
	if err != nil {
		return model.IssueRelayTicketResponse{}, err
	}
	ttl := time.Duration(req.TTLSeconds) * time.Second
	renewAfter := time.Duration(req.RenewAfterMs) * time.Millisecond
	ticket, err := s.store.IssueRelayTicket(req.PeerID, relay, ttl, renewAfter)
	if err != nil {
		return model.IssueRelayTicketResponse{}, err
	}
	return model.IssueRelayTicketResponse{Ticket: ticket}, nil
}

func (s *Service) GetDerpMap() (model.GetDerpMapResponse, error) {
	if s.biz != nil && s.biz.Enabled() {
		derpMap, err := s.biz.DerpMap(context.Background())
		if err != nil {
			return model.GetDerpMapResponse{}, fmt.Errorf("biz derp map unavailable: %w", err)
		}
		if len(preferredDerpCandidates(derpMap)) == 0 {
			return model.GetDerpMapResponse{}, ErrNoHealthyDerpNodes
		}
		return model.GetDerpMapResponse{Map: derpMap}, nil
	}
	return model.GetDerpMapResponse{Map: s.store.DerpMap()}, nil
}

func (s *Service) IssueDerpTicket(req model.IssueDerpTicketRequest) (model.IssueDerpTicketResponse, error) {
	if _, err := s.refreshPeerAuthorization(req.PeerID); err != nil {
		return model.IssueDerpTicketResponse{}, err
	}
	if s.biz != nil && s.biz.Enabled() {
		derpMap, err := s.biz.DerpMap(context.Background())
		if err != nil {
			return model.IssueDerpTicketResponse{}, fmt.Errorf("biz derp map unavailable: %w", err)
		}
		node, err := selectDerpNode(derpMap, req.RegionID, req.NodeID)
		if err != nil {
			return model.IssueDerpTicketResponse{}, err
		}
		req.RegionID = node.RegionID
		req.NodeID = node.NodeID
	}
	ttl := time.Duration(req.TTLSeconds) * time.Second
	renewAfter := time.Duration(req.RenewAfterMs) * time.Millisecond
	ticket, err := s.store.IssueDerpTicket(req.PeerID, req.RegionID, req.NodeID, ttl, renewAfter)
	if err != nil {
		return model.IssueDerpTicketResponse{}, err
	}
	return model.IssueDerpTicketResponse{Ticket: ticket}, nil
}

func (s *Service) BuildPathPlan(req model.PathPlanRequest) (model.PathPlan, error) {
	peerID := req.PeerID
	if peerID == "" {
		peerID = req.Peer.PeerID
	}
	if peerID == "" {
		return model.PathPlan{}, fmt.Errorf("peerId is required")
	}
	record, err := s.refreshPeerAuthorization(peerID)
	if err != nil {
		return model.PathPlan{}, err
	}
	snapshot := model.PeerSnapshot{
		PeerID:                  record.PeerID,
		ActivePath:              record.ActivePath,
		SupportsLANDirect:       record.SupportsLANDirect,
		SupportsIPv6Direct:      record.SupportsIPv6Direct,
		SupportsDirectUDP:       record.SupportsDirectUDP,
		SupportsRelayUDP:        record.SupportsRelayUDP,
		SupportsDerpTCPTLS443:   record.SupportsDerpTCPTLS443,
		EndpointChanged:         record.EndpointChanged,
		PreferIPv6:              record.PreferIPv6,
		PreferLAN:               record.PreferLAN,
		KeepaliveIntervalSecs:   record.KeepaliveIntervalSecs,
		Probes:                  record.Probes,
		RelayTicket:             record.RelayTicket,
		RecentPathDowngrades:    record.RecentPathDowngrades,
		RecentPathUpgrades:      record.RecentPathUpgrades,
		RequireMtuRefresh:       record.RequireMtuRefresh,
		AllowEndpointRoaming:    record.AllowEndpointRoaming,
		AllowFastReselection:    record.AllowFastReselection,
		AllowRelayTicketRenewal: record.AllowRelayTicketRenewal,
		DerpHealth:              append([]model.DerpHealthSample(nil), record.DerpHealth...),
	}
	plan := planner.BuildPlan(model.PathPlanRequest{Peer: snapshot})
	if s.biz != nil && s.biz.Enabled() {
		relayNodes, relayErr := s.relayNodes()
		if relayErr == nil && len(relayNodes) > 0 {
			plan.RelayCandidates = relayNodes
		} else {
			plan = removeRelayPath(plan)
			plan = appendDegradedReason(plan, degradedReason(relayErr, "relay"))
		}
	}
	derpMap, derpErr := s.derpMapForScheduling()
	if snapshot.SupportsDerpTCPTLS443 {
		if derpErr == nil {
			plan.DerpCandidates = preferredDerpCandidates(derpMap)
		}
		if s.biz != nil && s.biz.Enabled() && len(plan.DerpCandidates) == 0 {
			plan = removeDerpPath(plan)
			plan = appendDegradedReason(plan, degradedReason(derpErr, "derp"))
		}
		if plan.PreferredPath == model.PathDerpTCP443 && len(plan.DerpCandidates) > 0 {
			ticket, err := s.store.IssueDerpTicket(peerID, plan.DerpCandidates[0].RegionID, plan.DerpCandidates[0].NodeID, 5*time.Minute, time.Minute)
			if err == nil {
				plan.DerpTicket = &ticket
			}
		}
	}
	return plan, nil
}

func degradedReason(err error, kind string) string {
	if err == nil || errors.Is(err, ErrNoHealthyRelayNodes) || errors.Is(err, ErrNoHealthyDerpNodes) {
		return kind + "_no_healthy_nodes"
	}
	return kind + "_control_unavailable"
}

func appendDegradedReason(plan model.PathPlan, reason string) model.PathPlan {
	if reason == "" {
		return plan
	}
	if plan.DegradedReason == "" {
		plan.DegradedReason = reason
		return plan
	}
	plan.DegradedReason += "," + reason
	return plan
}

func (s *Service) GetPeer(peerID string) (model.PeerRecord, error) {
	return s.refreshPeerAuthorization(peerID)
}

func (s *Service) GetPeerAuthz(peerID string) (model.PeerAuthzView, error) {
	record, err := s.GetPeer(peerID)
	if err != nil {
		return model.PeerAuthzView{}, err
	}
	return model.PeerAuthzView{
		PeerID:      record.PeerID,
		NetworkID:   record.NetworkID,
		NodeID:      record.NodeID,
		Enabled:     true,
		VirtualIPs:  append([]string(nil), record.VirtualIPs...),
		AllowedIPs:  append([]string(nil), record.AllowedIPs...),
		QuotaPolicy: "default",
	}, nil
}

func (s *Service) GetPeerRuntimeConfig(peerID string) (model.PeerRuntimeConfigView, error) {
	record, err := s.GetPeer(peerID)
	if err != nil {
		return model.PeerRuntimeConfigView{}, err
	}
	var bizRuntime model.PeerRuntimeConfigView
	if s.biz != nil && s.biz.Enabled() {
		bizRuntime, err = s.biz.PeerRuntimeConfig(context.Background(), peerID)
		if err != nil {
			return model.PeerRuntimeConfigView{}, fmt.Errorf("biz runtime config denied: %w", err)
		}
		if !bizRuntime.NetworkEnabled {
			return model.PeerRuntimeConfigView{}, fmt.Errorf("peer network disabled by biz runtime config")
		}
	}
	plan, err := s.BuildPathPlan(model.PathPlanRequest{PeerID: peerID})
	if err != nil {
		return model.PeerRuntimeConfigView{}, err
	}
	virtualIPs := append([]string(nil), record.VirtualIPs...)
	allowedIPs := append([]string(nil), record.AllowedIPs...)
	if len(bizRuntime.VirtualIPs) > 0 {
		virtualIPs = append([]string(nil), bizRuntime.VirtualIPs...)
	}
	if len(bizRuntime.AllowedIPs) > 0 {
		allowedIPs = append([]string(nil), bizRuntime.AllowedIPs...)
	}
	return model.PeerRuntimeConfigView{
		PeerID:                record.PeerID,
		NetworkID:             record.NetworkID,
		NodeID:                record.NodeID,
		VirtualIPs:            virtualIPs,
		AllowedIPs:            allowedIPs,
		KeepaliveIntervalSecs: plan.Keepalive.IntervalSecs,
		NetworkEnabled:        true,
		PreferredPath:         plan.PreferredPath,
		Endpoints:             append([]model.Endpoint(nil), record.Endpoints...),
	}, nil
}

func (s *Service) GetNetworkTopology(networkID string) (model.NetworkTopologyView, error) {
	if s.biz != nil && s.biz.Enabled() {
		return s.biz.NetworkTopology(context.Background(), networkID)
	}
	peers := s.store.ListPeersByNetwork(networkID)
	if len(peers) == 0 {
		return model.NetworkTopologyView{}, store.ErrPeerNotFound
	}
	return model.NetworkTopologyView{
		NetworkID: networkID,
		Peers:     peers,
	}, nil
}

func (s *Service) refreshPeerAuthorization(peerID string) (model.PeerRecord, error) {
	record, ok := s.store.GetPeer(peerID)
	if !ok {
		return model.PeerRecord{}, store.ErrPeerNotFound
	}
	reg := record.PeerRegistration
	if err := s.applyBizAuthorization(context.Background(), &reg); err != nil {
		return model.PeerRecord{}, err
	}
	if s.biz == nil || !s.biz.Enabled() {
		return record, nil
	}
	return s.store.RegisterPeer(reg)
}

func (s *Service) derpMapForScheduling() (model.DerpMap, error) {
	if s.biz != nil && s.biz.Enabled() {
		derpMap, err := s.biz.DerpMap(context.Background())
		if err != nil {
			return model.DerpMap{}, err
		}
		if len(preferredDerpCandidates(derpMap)) == 0 {
			return model.DerpMap{}, ErrNoHealthyDerpNodes
		}
		return derpMap, nil
	}
	return s.store.DerpMap(), nil
}

func (s *Service) relayNodes() ([]model.RelayNode, error) {
	if s.biz == nil || !s.biz.Enabled() {
		return nil, nil
	}
	nodes, err := s.biz.RelayNodes(context.Background())
	if err != nil {
		return nil, err
	}
	out := make([]model.RelayNode, 0, len(nodes))
	for _, node := range nodes {
		if node.Enabled && node.Healthy && node.Host != "" && node.UDPPort > 0 {
			out = append(out, node)
		}
	}
	if len(out) == 0 {
		return nil, ErrNoHealthyRelayNodes
	}
	return out, nil
}

func (s *Service) primaryRelayNode() (model.RelayNode, error) {
	nodes, err := s.relayNodes()
	if err != nil {
		return model.RelayNode{}, err
	}
	if len(nodes) == 0 {
		return model.RelayNode{}, nil
	}
	return nodes[0], nil
}

func preferredDerpCandidates(m model.DerpMap) []model.DerpNode {
	out := make([]model.DerpNode, 0)
	for _, region := range m.Regions {
		if region.RegionID == m.PreferredRegionID {
			out = append(out, region.Nodes...)
		}
	}
	for _, region := range m.Regions {
		if region.RegionID == m.PreferredRegionID {
			continue
		}
		out = append(out, region.Nodes...)
	}
	return out
}

func selectDerpNode(m model.DerpMap, regionID, nodeID string) (model.DerpNode, error) {
	candidates := preferredDerpCandidates(m)
	for _, node := range candidates {
		if regionID != "" && node.RegionID != regionID {
			continue
		}
		if nodeID != "" && node.NodeID != nodeID {
			continue
		}
		return node, nil
	}
	return model.DerpNode{}, fmt.Errorf("biz derp node not found")
}

func removeRelayPath(plan model.PathPlan) model.PathPlan {
	return removeScheduledPath(plan, model.PathRelayUDP)
}

func removeDerpPath(plan model.PathPlan) model.PathPlan {
	plan = removeScheduledPath(plan, model.PathDerpTCP443)
	plan.DerpCandidates = nil
	plan.DerpTicket = nil
	return plan
}

func removeScheduledPath(plan model.PathPlan, target model.PathKind) model.PathPlan {
	plan.FallbackOrder = removePathKind(plan.FallbackOrder, target)
	filtered := plan.ScoredPaths[:0]
	for _, path := range plan.ScoredPaths {
		if path.Path != target {
			filtered = append(filtered, path)
		}
	}
	plan.ScoredPaths = filtered
	if plan.PreferredPath == target {
		plan.PreferredPath = ""
		if len(plan.ScoredPaths) > 0 {
			plan.ScoredPaths[0].Primary = true
			plan.PreferredPath = plan.ScoredPaths[0].Path
		} else if len(plan.FallbackOrder) > 0 {
			plan.PreferredPath = plan.FallbackOrder[0]
		}
	}
	if target == model.PathRelayUDP {
		plan.RelayTicket = model.RelayTicketPlan{}
		plan.RelayCandidates = nil
	}
	return plan
}

func removePathKind(paths []model.PathKind, target model.PathKind) []model.PathKind {
	out := paths[:0]
	for _, path := range paths {
		if path != target {
			out = append(out, path)
		}
	}
	return out
}
