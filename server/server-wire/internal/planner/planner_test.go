package planner

import (
	"testing"
	"time"

	"github.com/slan/server/server-wire/internal/model"
)

func TestBuildPlanPrefersLANThenIPv6ThenDirectThenRelay(t *testing.T) {
	plan := BuildPlan(model.PathPlanRequest{
		Peer: model.PeerSnapshot{
			SupportsLANDirect:     true,
			SupportsIPv6Direct:    true,
			SupportsDirectUDP:     true,
			SupportsRelayUDP:      true,
			SupportsDerpTCPTLS443: true,
			PreferLAN:             true,
			PreferIPv6:            true,
			Probes: []model.PathProbe{
				{Path: model.PathDerpTCP443, Reachable: true, RTTMs: 80},
				{Path: model.PathRelayUDP, Reachable: true, RTTMs: 30},
				{Path: model.PathDirectUDP, Reachable: true, RTTMs: 18},
				{Path: model.PathIPv6UDP, Reachable: true, RTTMs: 8},
				{Path: model.PathLANUDP, Reachable: true, RTTMs: 2},
			},
		},
	})

	if plan.PreferredPath != model.PathLANUDP {
		t.Fatalf("want lan path, got %s", plan.PreferredPath)
	}
	if len(plan.FallbackOrder) != 5 {
		t.Fatalf("want 5 paths, got %d", len(plan.FallbackOrder))
	}
	if plan.FallbackOrder[1] != model.PathIPv6UDP {
		t.Fatalf("want ipv6 as second path, got %s", plan.FallbackOrder[1])
	}
	if plan.FallbackOrder[4] != model.PathDerpTCP443 {
		t.Fatalf("want derp as final fallback, got %s", plan.FallbackOrder[4])
	}
}

func TestBuildPlanKeepsDerpAsFinalFallback(t *testing.T) {
	plan := BuildPlan(model.PathPlanRequest{
		Peer: model.PeerSnapshot{
			SupportsRelayUDP:      true,
			SupportsDerpTCPTLS443: true,
			Probes: []model.PathProbe{
				{Path: model.PathRelayUDP, Reachable: true, RTTMs: 30},
				{Path: model.PathDerpTCP443, Reachable: true, RTTMs: 20},
			},
		},
	})
	if len(plan.FallbackOrder) != 2 {
		t.Fatalf("want 2 fallback paths, got %d", len(plan.FallbackOrder))
	}
	if plan.FallbackOrder[0] != model.PathRelayUDP || plan.FallbackOrder[1] != model.PathDerpTCP443 {
		t.Fatalf("unexpected order: %#v", plan.FallbackOrder)
	}
}

func TestBuildPlanRequestsRelayRenewalNearExpiry(t *testing.T) {
	plan := BuildPlan(model.PathPlanRequest{
		Peer: model.PeerSnapshot{
			SupportsRelayUDP:        true,
			AllowRelayTicketRenewal: true,
			RelayTicket: model.RelayTicket{
				Present:     true,
				ExpiresInMs: 45_000,
			},
			Probes: []model.PathProbe{
				{Path: model.PathRelayUDP, Reachable: true, RTTMs: 40},
			},
		},
	})

	if !plan.RelayTicket.RenewRequired {
		t.Fatal("expected relay ticket renewal to be required")
	}
}

func TestBuildPlanIgnoresExpiredPathProbe(t *testing.T) {
	plan := BuildPlan(model.PathPlanRequest{
		Peer: model.PeerSnapshot{
			SupportsDirectUDP: true,
			SupportsRelayUDP:  true,
			Probes: []model.PathProbe{
				{Path: model.PathDirectUDP, Reachable: true, RTTMs: 1, ObservedAt: time.Now().Add(-10 * time.Minute).UnixMilli()},
				{Path: model.PathRelayUDP, Reachable: true, RTTMs: 50, ObservedAt: time.Now().UnixMilli()},
			},
		},
	})

	if plan.PreferredPath != model.PathRelayUDP {
		t.Fatalf("expected stale direct probe to be ignored, got %s", plan.PreferredPath)
	}
}
