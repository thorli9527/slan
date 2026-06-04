package relay

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/slan/server/server-wire-relay/internal/protocol"
	"github.com/slan/server/server-wire-relay/internal/state"
)

func TestHandleAttachAndForward(t *testing.T) {
	server := &UDPServer{store: state.NewStore()}
	a := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12001}
	b := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12002}
	ticket := protocol.RelayTicket{
		TicketID:  "t1",
		PeerID:    "peer-a",
		SessionID: "s1",
		Path:      "relay_udp",
		ExpiresAt: time.Now().Add(time.Minute),
	}
	ticket.Signature = signRelayTestTicket(ticket)

	attachA, _ := json.Marshal(protocol.ClientMessage{
		Kind:          "attach",
		ParticipantID: "node-a",
		Transport:     "udp",
		Ticket:        &ticket,
	})
	if err := server.handlePacketWithWriter(a, attachA, func(*net.UDPAddr, protocol.ServerMessage) error { return nil }); err != nil {
		t.Fatalf("attach a: %v", err)
	}

	attachB, _ := json.Marshal(protocol.ClientMessage{
		Kind:          "attach",
		ParticipantID: "node-b",
		Transport:     "udp",
		Ticket:        &ticket,
	})
	if err := server.handlePacketWithWriter(b, attachB, func(*net.UDPAddr, protocol.ServerMessage) error { return nil }); err != nil {
		t.Fatalf("attach b: %v", err)
	}

	var sentTo []*net.UDPAddr
	forward, _ := json.Marshal(protocol.ClientMessage{
		Kind:          "forward",
		SessionID:     "s1",
		ParticipantID: "node-a",
		Payload:       []byte("hello"),
	})
	if err := server.handlePacketWithWriter(a, forward, func(addr *net.UDPAddr, msg protocol.ServerMessage) error {
		sentTo = append(sentTo, addr)
		return nil
	}); err != nil {
		t.Fatalf("forward: %v", err)
	}
	if len(sentTo) != 1 {
		t.Fatalf("want peer packet write only, got %d", len(sentTo))
	}
	if sentTo[0].String() != b.String() {
		t.Fatalf("want first write to peer b, got %s", sentTo[0])
	}
}

func TestHandleForwardCanEmitCompatibilityAck(t *testing.T) {
	server := &UDPServer{store: state.NewStore()}
	server.cfg.ForwardAckEnabled = true
	a := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12011}
	b := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12012}
	ticket := protocol.RelayTicket{
		TicketID:  "t1",
		PeerID:    "peer-a",
		SessionID: "s1",
		Path:      "relay_udp",
		ExpiresAt: time.Now().Add(time.Minute),
	}
	ticket.Signature = signRelayTestTicket(ticket)

	for _, item := range []struct {
		addr *net.UDPAddr
		id   string
	}{{a, "node-a"}, {b, "node-b"}} {
		attach, _ := json.Marshal(protocol.ClientMessage{
			Kind:          "attach",
			ParticipantID: item.id,
			Transport:     "udp",
			Ticket:        &ticket,
		})
		if err := server.handlePacketWithWriter(item.addr, attach, func(*net.UDPAddr, protocol.ServerMessage) error { return nil }); err != nil {
			t.Fatalf("attach %s: %v", item.id, err)
		}
	}

	var kinds []string
	forward, _ := json.Marshal(protocol.ClientMessage{
		Kind:          "forward",
		SessionID:     "s1",
		ParticipantID: "node-a",
		Payload:       []byte("hello"),
	})
	if err := server.handlePacketWithWriter(a, forward, func(_ *net.UDPAddr, msg protocol.ServerMessage) error {
		kinds = append(kinds, msg.Kind)
		return nil
	}); err != nil {
		t.Fatalf("forward: %v", err)
	}
	if len(kinds) != 2 || kinds[0] != "packet" || kinds[1] != "forwarded" {
		t.Fatalf("unexpected writes: %#v", kinds)
	}
}

func signRelayTestTicket(ticket protocol.RelayTicket) string {
	payload := fmt.Sprintf("%s|%s|%s|%s|%s",
		ticket.TicketID,
		ticket.PeerID,
		ticket.SessionID,
		ticket.Path,
		ticket.ExpiresAt.UTC().Format(time.RFC3339Nano),
	)
	mac := hmac.New(sha256.New, []byte("dev-wire-ticket-secret"))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}
