package service

import (
	"context"
	"fmt"
	"time"

	"github.com/slan/server/server-wire/internal/model"
)

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
