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

func TestAttachRefreshesSessionExpiry(t *testing.T) {
	store := NewStore()
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 10001}
	firstExpiry := time.Now().Add(time.Minute).UTC()
	firstTicket := protocol.RelayTicket{
		TicketID:  "t1",
		PeerID:    "peer-a",
		SessionID: "s1",
		Path:      "relay_udp",
		ExpiresAt: firstExpiry,
	}
	firstTicket.Signature = signRelayTicket(firstTicket)
	if _, _, err := store.Attach(addr, "node-a", firstTicket, "udp"); err != nil {
		t.Fatalf("attach first ticket: %v", err)
	}

	secondExpiry := time.Now().Add(10 * time.Minute).UTC()
	secondTicket := firstTicket
	secondTicket.TicketID = "t2"
	secondTicket.ExpiresAt = secondExpiry
	secondTicket.Signature = signRelayTicket(secondTicket)
	session, _, err := store.Attach(addr, "node-a", secondTicket, "udp")
	if err != nil {
		t.Fatalf("attach renewal ticket: %v", err)
	}
	if !session.ExpiresAt.Equal(secondExpiry) {
		t.Fatalf("want refreshed expiry %s, got %s", secondExpiry, session.ExpiresAt)
	}
}

func TestExpiredSessionIsPrunedBeforeForward(t *testing.T) {
	store := NewStore()
	a := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 10001}
	b := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 10002}
	ticket := protocol.RelayTicket{
		TicketID:  "t1",
		PeerID:    "peer-a",
		SessionID: "s1",
		Path:      "relay_udp",
		ExpiresAt: time.Now().Add(30 * time.Millisecond),
	}
	ticket.Signature = signRelayTicket(ticket)
	if _, _, err := store.Attach(a, "node-a", ticket, "udp"); err != nil {
		t.Fatalf("attach a: %v", err)
	}
	if _, _, err := store.Attach(b, "node-b", ticket, "udp"); err != nil {
		t.Fatalf("attach b: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if _, _, err := store.Forward(a, "s1", "node-a", []byte("expired")); err != ErrSessionNotFound {
		t.Fatalf("expected expired session to be pruned, got %v", err)
	}
	if metrics := store.Metrics(); metrics.SessionCount != 0 || metrics.SourceBindingCount != 0 {
		t.Fatalf("expected expired state to be pruned, got %#v", metrics)
	}
}

func TestExpiredSourceBindingDoesNotBlockNewSession(t *testing.T) {
	store := NewStore()
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 10001}
	expiredSoon := protocol.RelayTicket{
		TicketID:  "t1",
		PeerID:    "peer-a",
		SessionID: "s1",
		Path:      "relay_udp",
		ExpiresAt: time.Now().Add(30 * time.Millisecond),
	}
	expiredSoon.Signature = signRelayTicket(expiredSoon)
	if _, _, err := store.Attach(addr, "node-a", expiredSoon, "udp"); err != nil {
		t.Fatalf("attach first session: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	next := protocol.RelayTicket{
		TicketID:  "t2",
		PeerID:    "peer-b",
		SessionID: "s2",
		Path:      "relay_udp",
		ExpiresAt: time.Now().Add(time.Minute),
	}
	next.Signature = signRelayTicket(next)
	if _, _, err := store.Attach(addr, "node-a", next, "udp"); err != nil {
		t.Fatalf("expired source binding should not block new session: %v", err)
	}
}

func TestAttachRejectsParticipantOutsideTicketEndpoints(t *testing.T) {
	store := NewStore()
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 10001}
	expiresAt := time.Now().Add(time.Minute).UTC().Truncate(time.Second)
	ticket := relayBusinessTicket("t1", "s1", "node-a", "node-b", expiresAt)

	if _, _, err := store.Attach(addr, "node-c", ticket, "udp"); err != ErrTicketInvalid {
		t.Fatalf("expected participant outside src/dst to be rejected, got %v", err)
	}
}

func TestAttachRejectsMismatchedPeerIDForBusinessTicket(t *testing.T) {
	store := NewStore()
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 10001}
	expiresAt := time.Now().Add(time.Minute).UTC().Truncate(time.Second)
	ticket := relayBusinessTicket("t1", "s1", "node-a", "node-b", expiresAt)
	ticket.PeerID = "node-c"

	if _, _, err := store.Attach(addr, "node-a", ticket, "udp"); err != ErrTicketInvalid {
		t.Fatalf("expected mismatched peerId to be rejected, got %v", err)
	}
}

func TestMetricsTrackRefreshAddressChangesAndForwarding(t *testing.T) {
	store := NewStore()
	a := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 10001}
	a2 := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 11001}
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
	if _, _, err := store.Attach(b, "node-b", ticket, "udp"); err != nil {
		t.Fatalf("attach b: %v", err)
	}
	if err := store.RefreshParticipant(a2, "s1", "node-a"); err != nil {
		t.Fatalf("refresh a: %v", err)
	}
	if _, _, err := store.Forward(a2, "s1", "node-a", []byte("hello")); err != nil {
		t.Fatalf("forward a: %v", err)
	}

	metrics := store.Metrics()
	if metrics.AttachCount != 2 {
		t.Fatalf("want attach count 2, got %#v", metrics)
	}
	if metrics.ParticipantRefreshCount != 1 {
		t.Fatalf("want refresh count 1, got %#v", metrics)
	}
	if metrics.ParticipantAddressChangeCount != 1 {
		t.Fatalf("want address change count 1, got %#v", metrics)
	}
	if metrics.ForwardCount != 1 {
		t.Fatalf("want forward count 1, got %#v", metrics)
	}
	if metrics.LastRefreshSessionID != "s1" || metrics.LastRefreshParticipantID != "node-a" {
		t.Fatalf("unexpected refresh identity in metrics: %#v", metrics)
	}
	if metrics.LastRefreshAddr != a2.String() {
		t.Fatalf("want refresh addr %s, got %#v", a2.String(), metrics)
	}
	if metrics.LastForwardSessionID != "s1" || metrics.LastForwardParticipantID != "node-a" {
		t.Fatalf("unexpected forward identity in metrics: %#v", metrics)
	}
	if metrics.LastForwardSourceAddr != a2.String() {
		t.Fatalf("want forward source addr %s, got %#v", a2.String(), metrics)
	}
	if metrics.LastForwardPeerID != "node-b" || metrics.LastForwardPeerAddr != b.String() {
		t.Fatalf("unexpected forward peer in metrics: %#v", metrics)
	}
}

func TestForwardRefreshesParticipantAddressOnRoam(t *testing.T) {
	store := NewStore()
	a := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 10001}
	a2 := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 11001}
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
	if _, _, err := store.Attach(b, "node-b", ticket, "udp"); err != nil {
		t.Fatalf("attach b: %v", err)
	}

	peerAddr, peerID, err := store.Forward(a2, "s1", "node-a", []byte("hello"))
	if err != nil {
		t.Fatalf("forward after roam: %v", err)
	}
	if peerID != "node-b" {
		t.Fatalf("want node-b, got %q", peerID)
	}
	if peerAddr.String() != b.String() {
		t.Fatalf("want %s, got %s", b.String(), peerAddr.String())
	}

	metrics := store.Metrics()
	if metrics.ParticipantRefreshCount != 1 {
		t.Fatalf("want refresh count 1, got %#v", metrics)
	}
	if metrics.ParticipantAddressChangeCount != 1 {
		t.Fatalf("want address change count 1, got %#v", metrics)
	}
	if metrics.LastRefreshAddr != a2.String() {
		t.Fatalf("want refresh addr %s, got %#v", a2.String(), metrics)
	}
	if metrics.LastForwardSourceAddr != a2.String() {
		t.Fatalf("want forward source addr %s, got %#v", a2.String(), metrics)
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

func relayBusinessTicket(ticketID, sessionID, srcNodeID, dstNodeID string, expiresAt time.Time) protocol.RelayTicket {
	ticket := protocol.RelayTicket{
		TicketID:  ticketID,
		NetworkID: "net-1",
		SessionID: sessionID,
		Path:      "relay_udp",
		SrcNodeID: srcNodeID,
		DstNodeID: dstNodeID,
		ExpiresAt: expiresAt,
	}
	ticket.ExpiresAtRaw = expiresAt.UTC().Format(time.RFC3339)
	ticket.Signature = signRelayBusinessTicket(ticket)
	return ticket
}

func signRelayBusinessTicket(ticket protocol.RelayTicket) string {
	payload := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		ticket.TicketID,
		ticket.NetworkID,
		ticket.SessionID,
		ticket.SrcNodeID,
		ticket.DstNodeID,
		ticket.ExpiresAtRaw,
	)
	mac := hmac.New(sha256.New, []byte("dev-relay-ticket-secret"))
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
