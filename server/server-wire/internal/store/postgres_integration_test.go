package store

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/slan/server/server-wire/internal/model"
)

func TestPostgresStorePersistsPeerRuntimeState(t *testing.T) {
	dsn := os.Getenv("SLAN_WIRE_POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("SLAN_WIRE_POSTGRES_TEST_DSN is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	first, err := NewPostgresStore(ctx, dsn)
	if err != nil {
		t.Fatalf("open first postgres store: %v", err)
	}
	defer first.Close()

	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	peerID := "peer-pg-" + suffix
	networkID := "net-pg-" + suffix
	if _, err := first.RegisterPeer(model.PeerRegistration{
		PeerID:                peerID,
		NetworkID:             networkID,
		NodeID:                "node-pg-" + suffix,
		VirtualIPs:            []string{"100.64.30.10"},
		AllowedIPs:            []string{"100.64.30.10/32"},
		SupportsDirectUDP:     true,
		SupportsRelayUDP:      true,
		SupportsDerpTCPTLS443: true,
	}); err != nil {
		t.Fatalf("register peer: %v", err)
	}
	if _, err := first.UpdatePathHealth(peerID, []model.PathProbe{{Path: model.PathDirectUDP, Reachable: true, RTTMs: 11, MTU: 1420}}); err != nil {
		t.Fatalf("update path health: %v", err)
	}
	if _, err := first.UpdateDerpHealth(peerID, []model.DerpHealthSample{{RegionID: "cn-east", NodeID: "derp-cn-east-1", Reachable: true, RTTMs: 35}}); err != nil {
		t.Fatalf("update derp health: %v", err)
	}
	if _, err := first.UpdateActivePath(peerID, model.PathDirectUDP); err != nil {
		t.Fatalf("update active path: %v", err)
	}
	relayTicket, err := first.IssueRelayTicket(peerID, model.RelayNode{}, time.Minute, 30*time.Second)
	if err != nil {
		t.Fatalf("issue relay ticket: %v", err)
	}
	if relayTicket.TicketID == "" || relayTicket.SessionID == "" || relayTicket.Signature == "" {
		t.Fatalf("invalid relay ticket: %#v", relayTicket)
	}

	second, err := NewPostgresStore(ctx, dsn)
	if err != nil {
		t.Fatalf("open second postgres store: %v", err)
	}
	defer second.Close()

	restored, ok := second.GetPeer(peerID)
	if !ok {
		t.Fatal("expected peer to be restored from postgres")
	}
	if restored.NetworkID != networkID || restored.ActivePath != model.PathDirectUDP {
		t.Fatalf("unexpected restored peer: %#v", restored)
	}
	if len(restored.Probes) != 1 || restored.Probes[0].Path != model.PathDirectUDP || restored.Probes[0].MTU != 1420 {
		t.Fatalf("unexpected restored probes: %#v", restored.Probes)
	}
	if len(restored.DerpHealth) != 1 || restored.DerpHealth[0].RegionID != "cn-east" {
		t.Fatalf("unexpected restored derp health: %#v", restored.DerpHealth)
	}
	if restored.RelayTicket.TicketID != relayTicket.TicketID || restored.RelayTicket.SessionID != relayTicket.SessionID {
		t.Fatalf("unexpected restored relay ticket: %#v want %#v", restored.RelayTicket, relayTicket)
	}
}
