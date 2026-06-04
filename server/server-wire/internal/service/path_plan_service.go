package service

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/slan/server/server-wire/internal/model"
	"github.com/slan/server/server-wire/internal/planner"
)

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
		if node.Enabled && node.Healthy && !node.Stale && node.Host != "" && node.UDPPort > 0 {
			out = append(out, node)
		}
	}
	if len(out) == 0 {
		return nil, ErrNoHealthyRelayNodes
	}
	sort.SliceStable(out, func(i, j int) bool {
		left := defaultPriority(out[i].Priority)
		right := defaultPriority(out[j].Priority)
		if left != right {
			return left < right
		}
		if out[i].RegionID != out[j].RegionID {
			return out[i].RegionID < out[j].RegionID
		}
		return out[i].NodeID < out[j].NodeID
	})
	return out, nil
}

func defaultPriority(value int) int {
	if value <= 0 {
		return 100
	}
	return value
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
