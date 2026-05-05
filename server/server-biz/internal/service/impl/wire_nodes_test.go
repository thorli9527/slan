package impl

import (
	"testing"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/repo"
)

func TestWireNodeSchedulableCountsExcludeStaleNodes(t *testing.T) {
	derp := []dto.WireDerpNodeRecord{
		{NodeID: "derp-ok", Host: "derp.local", Port: 443, Enabled: true, Healthy: true},
		{NodeID: "derp-stale", Host: "derp-stale.local", Port: 443, Enabled: true, Healthy: true, Stale: true},
	}
	if got := schedulableDerpCount(derp); got != 1 {
		t.Fatalf("schedulableDerpCount=%d want 1", got)
	}

	relay := []dto.WireRelayNodeRecord{
		{NodeID: "relay-ok", Host: "relay.local", UDPPort: 29110, Enabled: true, Healthy: true},
		{NodeID: "relay-stale", Host: "relay-stale.local", UDPPort: 29110, Enabled: true, Healthy: true, Stale: true},
	}
	if got := schedulableRelayCount(relay); got != 1 {
		t.Fatalf("schedulableRelayCount=%d want 1", got)
	}
}

func TestStaleWireNodeCountUsesDTOStaleFlag(t *testing.T) {
	derp := []dto.WireDerpNodeRecord{
		{NodeID: "derp-stale", Stale: true},
	}
	relay := []dto.WireRelayNodeRecord{
		{NodeID: "relay-stale", Stale: true},
	}
	if got := staleWireNodeCount(derp, relay); got != 2 {
		t.Fatalf("staleWireNodeCount=%d want 2", got)
	}
}

func TestWireNodeFreshnessUsesConfig(t *testing.T) {
	state := &dbState{cfg: configs.Config{Wire: configs.WireConfig{
		NodeHeartbeatFreshnessSeconds: 30,
		NodeCleanupIntervalSeconds:    5,
		NodeEventRetentionSeconds:     3600,
	}}}
	if got := state.wireNodeHeartbeatFreshnessWindow(); got != 30*time.Second {
		t.Fatalf("wireNodeHeartbeatFreshnessWindow=%s want 30s", got)
	}
	if got := state.wireNodeCleanupInterval(); got != 5*time.Second {
		t.Fatalf("wireNodeCleanupInterval=%s want 5s", got)
	}
	if got := state.wireNodeEventRetentionWindow(); got != time.Hour {
		t.Fatalf("wireNodeEventRetentionWindow=%s want 1h", got)
	}
	now := time.UnixMilli(100_000)
	if got := state.wireNodeFreshAfterMs(now); got != 70_000 {
		t.Fatalf("wireNodeFreshAfterMs=%d want 70000", got)
	}
}

func TestSummarizeWireTicketKeyHealthDetectsDrift(t *testing.T) {
	health := summarizeWireTicketKeyHealth([]dto.WireTicketKeyInstance{
		{Kind: "wire", NodeID: "wire-a", Status: dto.WireTicketKeyStatus{KeyRingID: "ring-a", RotationReady: true}},
		{Kind: "relay", NodeID: "relay-a", Status: dto.WireTicketKeyStatus{KeyRingID: "ring-a", RotationReady: true}},
		{Kind: "derp", NodeID: "derp-b", Status: dto.WireTicketKeyStatus{KeyRingID: "ring-b", RotationReady: true}},
	})
	if !health.Drifted || health.BaselineKeyRingID != "ring-a" {
		t.Fatalf("expected key ring drift, got %+v", health)
	}
	if !health.Instances[2].Drifted {
		t.Fatalf("expected derp-b to be marked drifted: %+v", health.Instances)
	}
	if !health.RotationReady {
		t.Fatalf("all available instances are rotation ready: %+v", health)
	}
}

func TestSummarizeWireTicketKeyHealthTracksUnavailableAndNotReady(t *testing.T) {
	health := summarizeWireTicketKeyHealth([]dto.WireTicketKeyInstance{
		{Kind: "wire", NodeID: "wire-a", Status: dto.WireTicketKeyStatus{KeyRingID: "ring-a", RotationReady: true}},
		{Kind: "relay", NodeID: "relay-a", Status: dto.WireTicketKeyStatus{Error: "timeout"}},
		{Kind: "derp", NodeID: "derp-a", Status: dto.WireTicketKeyStatus{KeyRingID: "ring-a", RotationReady: false}},
	})
	if health.Drifted || health.UnavailableCount != 1 || health.RotationReady {
		t.Fatalf("unexpected ticket key health: %+v", health)
	}
}

func TestWireNodeEventRetentionCanBeDisabled(t *testing.T) {
	state := &dbState{cfg: configs.Config{Wire: configs.WireConfig{NodeEventRetentionSeconds: -1}}}
	if got := state.wireNodeEventRetentionWindow(); got != 0 {
		t.Fatalf("wireNodeEventRetentionWindow=%s want disabled", got)
	}
}

func TestWireNodeEventBuilders(t *testing.T) {
	registered := wireNodeEventFromRelayUpsert(repo.WireRelayNode{
		RegionID: "local",
		NodeID:   "relay-a",
		Enabled:  true,
		Healthy:  true,
	}, repo.WireRelayNode{
		RegionID: "local",
		NodeID:   "relay-a",
		Enabled:  true,
		Healthy:  true,
	}, false)
	if registered.EventType != "registered" || registered.NodeKind != "relay" || registered.FromEnabled != nil || registered.ToEnabled == nil || !*registered.ToEnabled {
		t.Fatalf("unexpected registered event: %+v", registered)
	}

	updated := wireNodeEventFromDerpUpsert(repo.WireDerpNode{
		RegionID: "local",
		NodeID:   "derp-a",
		Enabled:  true,
		Healthy:  false,
	}, repo.WireDerpNode{
		RegionID: "local",
		NodeID:   "derp-a",
		Enabled:  true,
		Healthy:  true,
	}, true)
	if updated.EventType != "status_changed" || updated.NodeKind != "derp" || updated.FromHealthy == nil || *updated.FromHealthy || updated.ToHealthy == nil || !*updated.ToHealthy {
		t.Fatalf("unexpected updated event: %+v", updated)
	}

	stale := wireNodeHealthEvent("derp", "local", "derp-a", "stale_marked", true, true, true, false, "heartbeat_stale")
	if stale.EventType != "stale_marked" || stale.FromHealthy == nil || !*stale.FromHealthy || stale.ToHealthy == nil || *stale.ToHealthy {
		t.Fatalf("unexpected stale event: %+v", stale)
	}
}

func TestNormalizeWireNodeEventQuery(t *testing.T) {
	query := normalizeWireNodeEventQuery(dto.WireNodeEventQuery{
		NodeKind:  " DERP ",
		RegionID:  " local ",
		NodeID:    " derp-a ",
		EventType: " STALE_MARKED ",
		Page:      -1,
		PageSize:  500,
	})
	if query.NodeKind != "derp" || query.RegionID != "local" || query.NodeID != "derp-a" || query.EventType != "stale_marked" {
		t.Fatalf("unexpected normalized filters: %+v", query)
	}
	if query.Page != 1 || query.PageSize != 200 {
		t.Fatalf("unexpected normalized pagination: %+v", query)
	}
}
