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

	"github.com/slan/server/server-wire-derp/internal/protocol"
	"github.com/slan/server/server-wire-derp/internal/state"
)

func TestAdminRoutes(t *testing.T) {
	store := state.NewStore()
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
	ticket.Signature = signDerpTestTicket(ticket)
	_, _, err := store.Connect(serverConn, "peer-a", "node-a", "region-a", ticket)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}

	handler := New(store).Handler()
	for _, path := range []string{"/healthz", "/connections", "/connections/peer-a", "/sessions", "/regions", "/metrics"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("path=%s status=%d body=%s", path, rec.Code, rec.Body.String())
		}
	}
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
