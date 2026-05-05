package state

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/slan/server/server-wire-relay/internal/protocol"
)

func TestAttachForwardDetach(t *testing.T) {
	store := NewStore()
	a := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 10001}
	b := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 10002}
	ticket := protocol.RelayTicket{
		TicketID:  "t1",
		PeerID:    "peer-a",
		SessionID: "s1",
		Path:      "relay_udp",
		ExpiresAt: time.Now().Add(time.Minute),
	}
	ticket.Signature = signRelayTicket(ticket)

	if _, _, err := store.Attach(a, "node-a", ticket, "udp"); err != nil {
		t.Fatalf("attach a: %v", err)
	}
	if _, peer, err := store.Attach(b, "node-b", ticket, "relay_udp"); err != nil {
		t.Fatalf("attach b: %v", err)
	} else if peer != "node-a" {
		t.Fatalf("want node-a peer, got %q", peer)
	}
	peerAddr, peerID, err := store.Forward(a, "s1", "node-a", []byte("hello"))
	if err != nil {
		t.Fatalf("forward: %v", err)
	}
	if peerID != "node-b" {
		t.Fatalf("want node-b, got %q", peerID)
	}
	if peerAddr.String() != b.String() {
		t.Fatalf("want %s, got %s", b.String(), peerAddr.String())
	}
	if err := store.Detach(a, "s1", "node-a"); err != nil {
		t.Fatalf("detach: %v", err)
	}
}

func TestAttachRejectsExpiredTicket(t *testing.T) {
	store := NewStore()
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 10001}
	ticket := protocol.RelayTicket{
		TicketID:  "t1",
		PeerID:    "peer-a",
		SessionID: "s1",
		Path:      "relay_udp",
		ExpiresAt: time.Now().Add(-time.Minute),
	}
	ticket.Signature = signRelayTicket(ticket)
	_, _, err := store.Attach(addr, "node-a", ticket, "udp")
	if err != ErrTicketInvalid {
		t.Fatalf("want ErrTicketInvalid, got %v", err)
	}
}

func signRelayTicket(ticket protocol.RelayTicket) string {
	return signRelayTicketWithSecret(ticket, ticketSecret())
}

func signRelayTicketWithSecret(ticket protocol.RelayTicket, secret string) string {
	payload := fmt.Sprintf("%s|%s|%s|%s|%s",
		ticket.TicketID,
		ticket.PeerID,
		ticket.SessionID,
		ticket.Path,
		ticket.ExpiresAt.UTC().Format(time.RFC3339Nano),
	)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestAttachAcceptsPreviousTicketSecretDuringRotation(t *testing.T) {
	t.Setenv("SLAN_WIRE_TICKET_SECRETS", "new-secret,old-secret")
	store := NewStore()
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 10001}
	ticket := protocol.RelayTicket{
		TicketID:  "t1",
		PeerID:    "peer-a",
		SessionID: "s1",
		Path:      "relay_udp",
		ExpiresAt: time.Now().Add(time.Minute),
	}
	ticket.Signature = signRelayTicketWithSecret(ticket, "old-secret")
	if _, _, err := store.Attach(addr, "node-a", ticket, "udp"); err != nil {
		t.Fatalf("expected old rotation secret to validate: %v", err)
	}
}

func TestAttachRejectsRetiredTicketSecretAfterRotationCleanup(t *testing.T) {
	t.Setenv("SLAN_WIRE_TICKET_SECRETS", "new-secret")
	store := NewStore()
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 10001}
	ticket := protocol.RelayTicket{
		TicketID:  "t1",
		PeerID:    "peer-a",
		SessionID: "s1",
		Path:      "relay_udp",
		ExpiresAt: time.Now().Add(time.Minute),
	}
	ticket.Signature = signRelayTicketWithSecret(ticket, "old-secret")
	if _, _, err := store.Attach(addr, "node-a", ticket, "udp"); err != ErrTicketInvalid {
		t.Fatalf("expected retired rotation secret to be rejected, got %v", err)
	}
}

func TestAttachRejectsBadSignature(t *testing.T) {
	store := NewStore()
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 10001}
	_, _, err := store.Attach(addr, "node-a", protocol.RelayTicket{
		TicketID:  "t1",
		PeerID:    "peer-a",
		SessionID: "s1",
		Path:      "relay_udp",
		ExpiresAt: time.Now().Add(time.Minute),
		Signature: "bad",
	}, "udp")
	if err != ErrTicketInvalid {
		t.Fatalf("want ErrTicketInvalid, got %v", err)
	}
}

func TestTicketKeyStatusIncludesStableKeyRingID(t *testing.T) {
	t.Setenv("SLAN_WIRE_TICKET_SECRETS", "new-secret,old-secret")
	first := CurrentTicketKeyStatus()
	if first.KeyRingID == "" || !first.RotationReady || first.EffectiveKeyCount != 2 {
		t.Fatalf("unexpected key status: %#v", first)
	}

	t.Setenv("SLAN_WIRE_TICKET_SECRETS", "new-secret,different-old-secret")
	second := CurrentTicketKeyStatus()
	if second.KeyRingID == "" || second.EffectiveKeyCount != first.EffectiveKeyCount {
		t.Fatalf("unexpected changed key status: %#v", second)
	}
	if second.KeyRingID == first.KeyRingID {
		t.Fatalf("keyRingId must change when key material changes: first=%#v second=%#v", first, second)
	}
}
