package derp

import (
	"bufio"
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
		writers: make(map[string]*json.Encoder),
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
