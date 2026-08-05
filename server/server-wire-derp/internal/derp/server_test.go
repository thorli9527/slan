package derp

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/slan/server/server-wire-derp/internal/protocol"
	"github.com/slan/server/server-wire-derp/internal/state"
)

func TestHandleConnectAndDisconnect(t *testing.T) {
	server := &Server{
		store:   state.NewStore(),
		writers: make(map[string]*peerWriter),
	}
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()

	done := make(chan struct{})
	go func() {
		server.handleConn(serverConn)
		close(done)
	}()

	encoder := json.NewEncoder(clientConn)
	reader := bufio.NewScanner(clientConn)
	ticket := protocol.DerpTicket{
		TicketID:  "t1",
		PeerID:    "peer-a",
		Path:      "derp_tcp_tls_443",
		RegionID:  "region-a",
		NodeID:    "node-a",
		ExpiresAt: time.Now().Add(time.Minute),
	}
	ticket.Signature = signDerpTestTicket(ticket)
	err := encoder.Encode(protocol.ClientMessage{
		Kind:   "connect",
		PeerID: "peer-a",
		NodeID: "node-a",
		Ticket: &ticket,
	})
	if err != nil {
		t.Fatalf("encode connect: %v", err)
	}
	if !reader.Scan() {
		t.Fatal("expected connected response")
	}
	var connected protocol.ServerMessage
	if err := json.Unmarshal(reader.Bytes(), &connected); err != nil {
		t.Fatalf("decode connected: %v", err)
	}
	if connected.Kind != "connected" {
		t.Fatalf("want connected, got %s", connected.Kind)
	}

	if err := encoder.Encode(protocol.ClientMessage{Kind: "disconnect", SessionID: connected.SessionID}); err != nil {
		t.Fatalf("encode disconnect: %v", err)
	}
	if !reader.Scan() {
		t.Fatal("expected disconnected response")
	}
	<-done
}

func TestHandleSendForwardsBetweenRelayTicketPeers(t *testing.T) {
	server := &Server{
		store:   state.NewStore(),
		writers: make(map[string]*peerWriter),
	}
	aEncoder, _, closeA := connectDerpTestPeer(t, server, "peer-a", "peer-b")
	defer closeA()
	bEncoder, bReader, closeB := connectDerpTestPeer(t, server, "peer-b", "peer-a")
	defer closeB()

	payload := []byte("hello over derp")
	if err := aEncoder.Encode(protocol.ClientMessage{
		Kind:         "send",
		SessionID:    "relay-session-1",
		TargetPeerID: "peer-b",
		Payload:      payload,
	}); err != nil {
		t.Fatalf("encode send: %v", err)
	}
	recv := readDerpTestMessage(t, bReader)
	if recv.Kind != "recv" || recv.SessionID != "relay-session-1" || recv.SourcePeerID != "peer-a" || !bytes.Equal(recv.Payload, payload) {
		t.Fatalf("unexpected recv message: %#v", recv)
	}
	if err := bEncoder.Encode(protocol.ClientMessage{
		Kind:      "disconnect",
		SessionID: "relay-session-1",
	}); err != nil {
		t.Fatalf("disconnect peer-b: %v", err)
	}
}

func TestHandleSendCanEmitCompatibilityAck(t *testing.T) {
	server := &Server{
		store:   state.NewStore(),
		writers: make(map[string]*peerWriter),
	}
	server.cfg.SendAckEnabled = true
	aEncoder, aReader, closeA := connectDerpTestPeer(t, server, "peer-a", "peer-b")
	defer closeA()
	_, bReader, closeB := connectDerpTestPeer(t, server, "peer-b", "peer-a")
	defer closeB()

	payload := []byte("hello over derp")
	if err := aEncoder.Encode(protocol.ClientMessage{
		Kind:         "send",
		SessionID:    "relay-session-1",
		TargetPeerID: "peer-b",
		Payload:      payload,
	}); err != nil {
		t.Fatalf("encode send: %v", err)
	}
	recv := readDerpTestMessage(t, bReader)
	if recv.Kind != "recv" {
		t.Fatalf("unexpected recv: %#v", recv)
	}
	sent := readDerpTestMessage(t, aReader)
	if sent.Kind != "sent" || sent.BytesForwarded != len(payload) {
		t.Fatalf("unexpected sent ack: %#v", sent)
	}
}

func TestHandleSendReturnsErrorWhenTargetPeerMissing(t *testing.T) {
	server := &Server{
		store:   state.NewStore(),
		writers: make(map[string]*peerWriter),
	}
	encoder, reader, closePeer := connectDerpTestPeer(t, server, "peer-a", "peer-b")
	defer closePeer()

	if err := encoder.Encode(protocol.ClientMessage{
		Kind:         "send",
		SessionID:    "relay-session-1",
		TargetPeerID: "peer-b",
		Payload:      []byte("missing target"),
	}); err != nil {
		t.Fatalf("encode send: %v", err)
	}
	msg := readDerpTestMessage(t, reader)
	if msg.Kind != "error" || msg.Error == nil || msg.Error.Code != "target_not_connected" {
		t.Fatalf("expected target_not_connected, got %#v", msg)
	}
}

func TestHandleSendRequiresConnect(t *testing.T) {
	server := &Server{
		store:   state.NewStore(),
		writers: make(map[string]*peerWriter),
	}
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	go server.handleConn(serverConn)

	encoder := json.NewEncoder(clientConn)
	reader := bufio.NewScanner(clientConn)
	if err := encoder.Encode(protocol.ClientMessage{
		Kind:         "send",
		SessionID:    "relay-session-1",
		TargetPeerID: "peer-b",
		Payload:      []byte("unauthenticated"),
	}); err != nil {
		t.Fatalf("encode send: %v", err)
	}
	msg := readDerpTestMessage(t, reader)
	if msg.Kind != "error" || msg.Error == nil || msg.Error.Code != "connect_required" {
		t.Fatalf("expected connect_required, got %#v", msg)
	}
}

func TestClearWriterKeepsNewerPeerConnection(t *testing.T) {
	server := &Server{
		writers: make(map[string]*peerWriter),
	}
	oldWriter := &peerWriter{encoder: json.NewEncoder(&bytes.Buffer{})}
	newWriter := &peerWriter{encoder: json.NewEncoder(&bytes.Buffer{})}

	server.setWriter("peer-a", oldWriter)
	server.setWriter("peer-a", newWriter)
	if server.clearWriter("peer-a", oldWriter) {
		t.Fatalf("old writer should not clear newer connection")
	}

	current, ok := server.writer("peer-a")
	if !ok || current != newWriter {
		t.Fatalf("expected newer writer to remain registered")
	}

	if !server.clearWriter("peer-a", newWriter) {
		t.Fatalf("current writer should be cleared")
	}
	if _, ok := server.writer("peer-a"); ok {
		t.Fatalf("expected current writer to be cleared")
	}
}

func TestOldPeerConnectionCloseDoesNotDisconnectNewerConnection(t *testing.T) {
	store := state.NewStore()
	server := &Server{
		store:   store,
		writers: make(map[string]*peerWriter),
	}
	_, _, closeOld := connectDerpTestPeer(t, server, "peer-a", "peer-b")
	_, _, closeNew := connectDerpTestPeer(t, server, "peer-a", "peer-b")
	defer closeNew()

	closeOld()
	time.Sleep(20 * time.Millisecond)

	if _, ok := server.writer("peer-a"); !ok {
		t.Fatalf("newer writer should remain registered")
	}
	if _, ok := store.Connection("peer-a"); !ok {
		t.Fatalf("newer peer connection should remain in state")
	}
}

func TestForwardReachesNewPeerConnectionAfterOldConnectionCloses(t *testing.T) {
	store := state.NewStore()
	server := &Server{
		store:   store,
		writers: make(map[string]*peerWriter),
	}
	aEncoder, _, closeA := connectDerpTestPeer(t, server, "peer-a", "peer-b")
	defer closeA()
	_, _, closeOldB := connectDerpTestPeer(t, server, "peer-b", "peer-a")
	_, newBReader, closeNewB := connectDerpTestPeer(t, server, "peer-b", "peer-a")
	defer closeNewB()

	closeOldB()
	time.Sleep(20 * time.Millisecond)

	payload := []byte("forward after reconnect")
	if err := aEncoder.Encode(protocol.ClientMessage{
		Kind:         "send",
		SessionID:    "relay-session-1",
		TargetPeerID: "peer-b",
		Payload:      payload,
	}); err != nil {
		t.Fatalf("encode send after reconnect: %v", err)
	}
	recv := readDerpTestMessage(t, newBReader)
	if recv.Kind != "recv" || recv.SourcePeerID != "peer-a" || !bytes.Equal(recv.Payload, payload) {
		t.Fatalf("unexpected message on new connection: %#v", recv)
	}
}

func TestSupersededPeerConnectionCannotSend(t *testing.T) {
	store := state.NewStore()
	server := &Server{
		store:   store,
		writers: make(map[string]*peerWriter),
	}
	oldEncoder, oldReader, closeOld := connectDerpTestPeer(t, server, "peer-a", "peer-b")
	defer closeOld()
	_, _, closeTarget := connectDerpTestPeer(t, server, "peer-b", "peer-a")
	defer closeTarget()
	_, _, closeNew := connectDerpTestPeer(t, server, "peer-a", "peer-b")
	defer closeNew()

	if err := oldEncoder.Encode(protocol.ClientMessage{
		Kind:         "send",
		SessionID:    "relay-session-1",
		TargetPeerID: "peer-b",
		Payload:      []byte("stale connection"),
	}); err != nil {
		t.Fatalf("encode stale send: %v", err)
	}
	msg := readDerpTestMessage(t, oldReader)
	if msg.Kind != "error" || msg.Error == nil || msg.Error.Code != "connection_superseded" {
		t.Fatalf("expected connection_superseded, got %#v", msg)
	}
	if _, ok := server.writer("peer-a"); !ok {
		t.Fatalf("newer writer should remain registered")
	}
	if _, ok := store.Connection("peer-a"); !ok {
		t.Fatalf("newer connection should remain in state")
	}
}

func connectDerpTestPeer(t *testing.T, server *Server, peerID, targetPeerID string) (*json.Encoder, *bufio.Scanner, func()) {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	go server.handleConn(serverConn)
	encoder := json.NewEncoder(clientConn)
	reader := bufio.NewScanner(clientConn)
	expiresAt := time.Now().Add(time.Minute).UTC().Truncate(time.Second)
	ticket := relayDerpTestTicket("ticket-"+peerID, "relay-session-1", peerID, peerID, targetPeerID, expiresAt)
	if err := encoder.Encode(protocol.ClientMessage{
		Kind:     "connect",
		PeerID:   peerID,
		NodeID:   "node-a",
		RegionID: "region-a",
		Ticket:   &ticket,
	}); err != nil {
		t.Fatalf("encode connect: %v", err)
	}
	connected := readDerpTestMessage(t, reader)
	if connected.Kind != "connected" || connected.SessionID != "relay-session-1" {
		t.Fatalf("expected connected relay session, got %#v", connected)
	}
	return encoder, reader, func() { _ = clientConn.Close() }
}

func readDerpTestMessage(t *testing.T, reader *bufio.Scanner) protocol.ServerMessage {
	t.Helper()
	if !reader.Scan() {
		t.Fatalf("expected DERP message, scan err=%v", reader.Err())
	}
	var msg protocol.ServerMessage
	if err := json.Unmarshal(reader.Bytes(), &msg); err != nil {
		t.Fatalf("decode DERP message: %v", err)
	}
	return msg
}

func signDerpTestTicket(ticket protocol.DerpTicket) string {
	payload := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s",
		ticket.TicketID,
		ticket.PeerID,
		ticket.NetworkID,
		ticket.Path,
		ticket.RegionID,
		ticket.NodeID,
		ticket.ExpiresAt.UTC().Format(time.RFC3339Nano),
	)
	mac := hmac.New(sha256.New, []byte("dev-wire-ticket-secret"))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func relayDerpTestTicket(ticketID, sessionID, peerID, srcNodeID, dstNodeID string, expiresAt time.Time) protocol.DerpTicket {
	ticket := protocol.DerpTicket{
		TicketID:  ticketID,
		PeerID:    peerID,
		NetworkID: "net-1",
		Path:      "derp_tcp_tls_443",
		RegionID:  "region-a",
		NodeID:    "node-a",
		SessionID: sessionID,
		SrcNodeID: srcNodeID,
		DstNodeID: dstNodeID,
		ExpiresAt: expiresAt,
	}
	ticket.Signature = signRelayDerpTestTicket(ticket)
	return ticket
}

func signRelayDerpTestTicket(ticket protocol.DerpTicket) string {
	payload := fmt.Sprintf("%s|%s|%s|%s|%s|%s",
		ticket.TicketID,
		ticket.NetworkID,
		ticket.SessionID,
		ticket.SrcNodeID,
		ticket.DstNodeID,
		ticket.ExpiresAt.UTC().Format(time.RFC3339),
	)
	mac := hmac.New(sha256.New, []byte("dev-relay-ticket-secret"))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
