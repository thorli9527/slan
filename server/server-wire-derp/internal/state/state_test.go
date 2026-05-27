package state

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/slan/server/server-wire-derp/internal/protocol"
)

func TestConnectAndBindSession(t *testing.T) {
	store := NewStore()
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	expiresAt := time.Now().Add(time.Minute)
	ticket := protocol.DerpTicket{
		TicketID:  "t1",
		PeerID:    "peer-a",
		Path:      "derp_tcp_tls_443",
		RegionID:  "region-a",
		NodeID:    "node-a",
		ExpiresAt: expiresAt,
	}
	ticket.Signature = signDerpTestTicket(ticket)
	session, renewAfter, err := store.Connect(serverConn, "peer-a", "node-a", "region-a", ticket)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if session.SessionID == "" || renewAfter <= 0 {
		t.Fatalf("unexpected session=%#v renewAfter=%d", session, renewAfter)
	}
	session, err = store.BindSessionPeer(session.SessionID, "peer-b")
	if err != nil {
		t.Fatalf("bind peer: %v", err)
	}
	if session.PeerB != "peer-b" {
		t.Fatalf("want peer-b, got %q", session.PeerB)
	}
}

func TestDisconnectTargetPeerKeepsOwnerSession(t *testing.T) {
	store := NewStore()
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	expiresAt := time.Now().Add(time.Minute)
	ticket := protocol.DerpTicket{
		TicketID:  "t1",
		PeerID:    "peer-a",
		Path:      "derp_tcp_tls_443",
		RegionID:  "region-a",
		NodeID:    "node-a",
		ExpiresAt: expiresAt,
	}
	ticket.Signature = signDerpTestTicket(ticket)
	session, _, err := store.Connect(serverConn, "peer-a", "node-a", "region-a", ticket)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := store.BindSessionPeer(session.SessionID, "peer-b"); err != nil {
		t.Fatalf("bind peer: %v", err)
	}

	store.Disconnect("peer-b")

	view, ok := store.Session(session.SessionID)
	if !ok {
		t.Fatalf("owner session should survive target peer disconnect")
	}
	if view.PeerB != "" {
		t.Fatalf("target peer binding should be cleared, got %q", view.PeerB)
	}
}

func TestConnectUsesUniqueRandomSessionIDs(t *testing.T) {
	store := NewStore()
	firstServer, firstClient := net.Pipe()
	defer firstServer.Close()
	defer firstClient.Close()
	secondServer, secondClient := net.Pipe()
	defer secondServer.Close()
	defer secondClient.Close()

	ticket := protocol.DerpTicket{
		TicketID:  "t1",
		PeerID:    "peer-a",
		Path:      "derp_tcp_tls_443",
		RegionID:  "region-a",
		NodeID:    "node-a",
		ExpiresAt: time.Now().Add(time.Minute),
	}
	ticket.Signature = signDerpTestTicket(ticket)
	first, _, err := store.Connect(firstServer, "peer-a", "node-a", "region-a", ticket)
	if err != nil {
		t.Fatalf("first connect: %v", err)
	}
	ticket.TicketID = "t2"
	ticket.Signature = signDerpTestTicket(ticket)
	second, _, err := store.Connect(secondServer, "peer-a", "node-a", "region-a", ticket)
	if err != nil {
		t.Fatalf("second connect: %v", err)
	}
	if first.SessionID == second.SessionID {
		t.Fatalf("session IDs must be unique: %q", first.SessionID)
	}
	if !strings.HasPrefix(first.SessionID, "derp-session-") {
		t.Fatalf("unexpected session ID: %q", first.SessionID)
	}
}

func signDerpTestTicket(ticket protocol.DerpTicket) string {
	return signDerpTestTicketWithSecret(ticket, "dev-wire-ticket-secret")
}

func signDerpTestTicketWithSecret(ticket protocol.DerpTicket, secret string) string {
	payload := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s",
		ticket.TicketID,
		ticket.PeerID,
		ticket.NetworkID,
		ticket.Path,
		ticket.RegionID,
		ticket.NodeID,
		ticket.ExpiresAt.UTC().Format(time.RFC3339Nano),
	)
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestConnectAcceptsPreviousTicketSecretDuringRotation(t *testing.T) {
	t.Setenv("SLAN_WIRE_TICKET_SECRETS", "new-secret,old-secret")
	store := NewStore()
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	ticket := protocol.DerpTicket{
		TicketID:  "t1",
		PeerID:    "peer-a",
		Path:      "derp_tcp_tls_443",
		RegionID:  "region-a",
		NodeID:    "node-a",
		ExpiresAt: time.Now().Add(time.Minute),
	}
	ticket.Signature = signDerpTestTicketWithSecret(ticket, "old-secret")
	if _, _, err := store.Connect(serverConn, "peer-a", "node-a", "region-a", ticket); err != nil {
		t.Fatalf("expected old rotation secret to validate: %v", err)
	}
}

func TestConnectRejectsRetiredTicketSecretAfterRotationCleanup(t *testing.T) {
	t.Setenv("SLAN_WIRE_TICKET_SECRETS", "new-secret")
	store := NewStore()
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	ticket := protocol.DerpTicket{
		TicketID:  "t1",
		PeerID:    "peer-a",
		Path:      "derp_tcp_tls_443",
		RegionID:  "region-a",
		NodeID:    "node-a",
		ExpiresAt: time.Now().Add(time.Minute),
	}
	ticket.Signature = signDerpTestTicketWithSecret(ticket, "old-secret")
	if _, _, err := store.Connect(serverConn, "peer-a", "node-a", "region-a", ticket); err != ErrTicketInvalid {
		t.Fatalf("expected retired rotation secret to be rejected, got %v", err)
	}
}

func TestConnectRejectsBadSignature(t *testing.T) {
	store := NewStore()
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	_, _, err := store.Connect(serverConn, "peer-a", "node-a", "region-a", protocol.DerpTicket{
		TicketID:  "t1",
		PeerID:    "peer-a",
		Path:      "derp_tcp_tls_443",
		RegionID:  "region-a",
		NodeID:    "node-a",
		ExpiresAt: time.Now().Add(time.Minute),
		Signature: "bad",
	})
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
