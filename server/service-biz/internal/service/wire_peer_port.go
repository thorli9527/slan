package service

import (
	"context"

	"github.com/slan/service-biz/internal/pkg/wirekit"
)

type WirePeerAuthzView = wirekit.PeerAuthz
type WirePeerRuntimeConfigView = wirekit.PeerRuntimeConfig
type WireTopologyView = wirekit.Topology

type WirePathProbeInput struct {
	Path       string `json:"path"`
	Reachable  bool   `json:"reachable"`
	RTTMs      int    `json:"rttMs"`
	LossPPM    int    `json:"lossPpm"`
	MTU        int    `json:"mtu"`
	ObservedAt int64  `json:"observedAt"`
}

type WirePeerPathHealthInput struct {
	PeerID string               `json:"peerId"`
	Probes []WirePathProbeInput `json:"probes"`
}

type WirePeerUseCase interface {
	PeerAuthz(ctx context.Context, peerID string) (WirePeerAuthzView, error)
	PeerRuntimeConfig(ctx context.Context, peerID string) (WirePeerRuntimeConfigView, error)
	NetworkTopology(ctx context.Context, networkID string) (WireTopologyView, error)
	ReportPeerPathHealth(ctx context.Context, input WirePeerPathHealthInput) error
}
