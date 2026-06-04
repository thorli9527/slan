package admin

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/slan/server/server-wire-relay/internal/protocol"
	"github.com/slan/server/server-wire-relay/internal/state"
)

func TestAdminRoutes(t *testing.T) {
	store := state.NewStore()
	ticket := protocol.RelayTicket{
		TicketID:  "t1",
		PeerID:    "peer-a",
		SessionID: "s1",
		Path:      "relay_udp",
		ExpiresAt: time.Now().Add(time.Minute),
	}
	ticket.Signature = signRelayTestTicket(ticket)
	_, _, err := store.Attach(
		&net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 10001},
		"node-a",
		ticket,
		"udp",
	)
	if err != nil {
		t.Fatalf("attach: %v", err)
	}

	handler := New(store).Handler()
	for _, path := range []string{"/healthz", "/sessions", "/sessions/s1", "/metrics"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("path=%s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
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
