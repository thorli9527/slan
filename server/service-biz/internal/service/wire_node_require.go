package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

func (s WireNodeService) requireRelayNodeRegion(ctx context.Context, regionID, nodeID string) (model.RelayNode, error) {
	item, ok, err := s.Catalog.GetRelayNode(ctx, normalizeWireNodeID(nodeID))
	if err != nil {
		return model.RelayNode{}, err
	}
	if !ok {
		return model.RelayNode{}, ErrNotFound
	}
	if expected := normalizeWireRegion(regionID); expected != "" && item.Region != expected {
		return model.RelayNode{}, ErrNotFound
	}
	if item.Transport != "" && item.Transport != "relay_udp" {
		return model.RelayNode{}, ErrNotFound
	}
	return item, nil
}

func (s WireNodeService) requireDerpNodeRegion(ctx context.Context, regionID, nodeID string) (model.RelayNode, error) {
	item, ok, err := s.Catalog.GetRelayNode(ctx, normalizeWireNodeID(nodeID))
	if err != nil {
		return model.RelayNode{}, err
	}
	if !ok {
		return model.RelayNode{}, ErrNotFound
	}
	if expected := normalizeWireRegion(regionID); expected != "" && item.Region != expected {
		return model.RelayNode{}, ErrNotFound
	}
	if item.Transport != "derp_tcp_tls_443" {
		return model.RelayNode{}, ErrNotFound
	}
	return item, nil
}
