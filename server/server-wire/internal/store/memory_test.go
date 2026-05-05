package store

import (
	"strings"
	"testing"

	"github.com/slan/server/server-wire/internal/model"
)

func TestIssueTicketsUseUniqueRandomIDs(t *testing.T) {
	st := NewMemoryStore()
	if _, err := st.RegisterPeer(model.PeerRegistration{
		PeerID:                "peer-ticket",
		NetworkID:             "net-ticket",
		SupportsRelayUDP:      true,
		SupportsDerpTCPTLS443: true,
	}); err != nil {
		t.Fatalf("register peer: %v", err)
	}

	relayA, err := st.IssueRelayTicket("peer-ticket", model.RelayNode{}, 0, 0)
	if err != nil {
		t.Fatalf("issue relay A: %v", err)
	}
	relayB, err := st.IssueRelayTicket("peer-ticket", model.RelayNode{}, 0, 0)
	if err != nil {
		t.Fatalf("issue relay B: %v", err)
	}
	if relayA.TicketID == relayB.TicketID {
		t.Fatalf("relay ticket IDs must be unique: %q", relayA.TicketID)
	}
	if relayA.SessionID == relayB.SessionID {
		t.Fatalf("relay session IDs must be unique: %q", relayA.SessionID)
	}
	if !strings.HasPrefix(relayA.TicketID, "relay-") || !strings.HasPrefix(relayA.SessionID, "relay-session-") {
		t.Fatalf("unexpected relay IDs: ticket=%q session=%q", relayA.TicketID, relayA.SessionID)
	}

	derpA, err := st.IssueDerpTicket("peer-ticket", "", "", 0, 0)
	if err != nil {
		t.Fatalf("issue derp A: %v", err)
	}
	derpB, err := st.IssueDerpTicket("peer-ticket", "", "", 0, 0)
	if err != nil {
		t.Fatalf("issue derp B: %v", err)
	}
	if derpA.TicketID == derpB.TicketID {
		t.Fatalf("derp ticket IDs must be unique: %q", derpA.TicketID)
	}
	if !strings.HasPrefix(derpA.TicketID, "derp-") {
		t.Fatalf("unexpected derp ticket ID: %q", derpA.TicketID)
	}
}

func TestTicketKeyStatusIncludesStableKeyRingID(t *testing.T) {
	t.Setenv("SLAN_WIRE_TICKET_SECRETS", "new-secret,old-secret")
	first := TicketKeyStatus()
	if first.KeyRingID == "" || !first.RotationReady || first.EffectiveKeyCount != 2 {
		t.Fatalf("unexpected key status: %#v", first)
	}

	t.Setenv("SLAN_WIRE_TICKET_SECRETS", "new-secret,different-old-secret")
	second := TicketKeyStatus()
	if second.KeyRingID == "" || second.EffectiveKeyCount != first.EffectiveKeyCount {
		t.Fatalf("unexpected changed key status: %#v", second)
	}
	if second.KeyRingID == first.KeyRingID {
		t.Fatalf("keyRingId must change when key material changes: first=%#v second=%#v", first, second)
	}
}
