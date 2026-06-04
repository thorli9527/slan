package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/slan/server/server-wire/internal/model"
	"github.com/slan/server/server-wire/internal/store"
)

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
	normalizePathProbeObservedAt(req.Probes, time.Now().UnixMilli())
	return s.store.UpdatePathHealth(req.PeerID, req.Probes)
}

func (s *Service) ReportDerpHealth(req model.ReportDerpHealthRequest) (model.PeerRecord, error) {
	if _, err := s.refreshPeerAuthorization(req.PeerID); err != nil {
		return model.PeerRecord{}, err
	}
	return s.store.UpdateDerpHealth(req.PeerID, req.Samples)
}

func normalizePathProbeObservedAt(probes []model.PathProbe, nowMs int64) {
	for i := range probes {
		if probes[i].ObservedAt <= 0 {
			probes[i].ObservedAt = nowMs
			continue
		}
		if probes[i].ObservedAt < 1_000_000_000_000 {
			probes[i].ObservedAt *= 1000
		}
	}
}

func (s *Service) UpdateActivePath(req model.UpdateActivePathRequest) (model.PeerRecord, error) {
	if _, err := s.refreshPeerAuthorization(req.PeerID); err != nil {
		return model.PeerRecord{}, err
	}
	return s.store.UpdateActivePath(req.PeerID, req.Path)
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
