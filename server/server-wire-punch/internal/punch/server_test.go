package punch

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/slan/server/server-wire-punch/internal/config"
)

func TestEndpointLifecycleAndConnectSession(t *testing.T) {
	server := &Server{
		store: NewStore(),
		cfg: config.Config{
			EndpointTTL: 2 * time.Minute,
			SessionTTL:  time.Minute,
		},
	}
	handler := server.Handler()

	postEndpoint(t, handler, map[string]any{
		"networkId": "net-a",
		"nodeId":    "node-a",
		"type":      "reflexive",
		"address":   "203.0.113.10:40000",
	})
	postEndpoint(t, handler, map[string]any{
		"networkId": "net-a",
		"nodeId":    "node-b",
		"type":      "reflexive",
		"address":   "203.0.113.11:40001",
	})

	body := requestJSON(t, handler, http.MethodPost, "/connect-sessions", map[string]any{
		"networkId":       "net-a",
		"requesterNodeId": "node-a",
		"peerNodeId":      "node-b",
	})
	var session ConnectSession
	if err := json.Unmarshal(body, &session); err != nil {
		t.Fatal(err)
	}
	if session.SessionID == "" || session.Requester == nil || session.Peer == nil {
		t.Fatalf("unexpected session: %+v", session)
	}
	if session.Peer.Address != "203.0.113.11:40001" {
		t.Fatalf("unexpected peer endpoint: %+v", session.Peer)
	}
}

func TestUDPEndpointProbeReflectsRemoteAddress(t *testing.T) {
	conn, err := net.ListenUDP("udp", mustUDPAddr(t, "127.0.0.1:0"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	server := &Server{
		conn:  conn,
		store: NewStore(),
		cfg: config.Config{
			EndpointTTL: 2 * time.Minute,
			SessionTTL:  time.Minute,
		},
	}
	client, err := net.ListenUDP("udp", mustUDPAddr(t, "127.0.0.1:0"))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	payload, _ := json.Marshal(UDPMessage{
		Kind:      "endpoint_probe",
		NetworkID: "net-a",
		NodeID:    "node-a",
	})
	if err := server.handlePacket(client.LocalAddr().(*net.UDPAddr), payload); err != nil {
		t.Fatal(err)
	}

	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	buf := make([]byte, 2048)
	n, _, err := client.ReadFromUDP(buf)
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Kind     string   `json:"kind"`
		Endpoint Endpoint `json:"endpoint"`
	}
	if err := json.Unmarshal(buf[:n], &response); err != nil {
		t.Fatal(err)
	}
	if response.Kind != "endpoint_reflexive" || response.Endpoint.Reflexive == "" {
		t.Fatalf("unexpected response: %+v", response)
	}
}

func TestStatsExpiresOldState(t *testing.T) {
	store := NewStore()
	now := time.Now()
	store.PutEndpoint(Endpoint{
		NetworkID: "net-a",
		NodeID:    "node-a",
		Address:   "127.0.0.1:1",
		UpdatedAt: now.Add(-time.Minute),
		ExpiresAt: now.Add(-time.Second),
	})
	stats := store.Stats()
	if stats.EndpointCount != 0 {
		t.Fatalf("expected expired endpoint cleanup, got %+v", stats)
	}
}

func TestMutatingHTTPRequiresDeviceTokenWhenInternalTokenIsSet(t *testing.T) {
	server := &Server{
		store: NewStore(),
		cfg: config.Config{
			InternalWireToken: "wire-token",
			EndpointTTL:       2 * time.Minute,
			SessionTTL:        time.Minute,
		},
	}
	handler := server.Handler()
	payload, _ := json.Marshal(map[string]any{
		"networkId": "net-a",
		"nodeId":    "node-a",
		"address":   "203.0.113.10:40000",
	})
	req := httptest.NewRequest(http.MethodPost, "/endpoints", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slan-Internal-Token", "wire-token")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected missing device token to be rejected, got status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/endpoints", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slan-Device-ID", "node-a")
	req.Header.Set("X-Slan-MQTT-Username", "slan:node-a:4102444800")
	req.Header.Set("X-Slan-Punch-Signature", "signature")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected missing internal token to be rejected, got status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPost, "/endpoints", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Slan-Internal-Token", "wire-token")
	req.Header.Set("X-Slan-Device-ID", "node-a")
	req.Header.Set("X-Slan-MQTT-Username", "slan:node-a:4102444800")
	req.Header.Set("X-Slan-Punch-Signature", "signature")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("expected device token to be accepted, got status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func postEndpoint(t *testing.T, handler http.Handler, body map[string]any) {
	t.Helper()
	_ = requestJSON(t, handler, http.MethodPost, "/endpoints", body)
}

func requestJSON(t *testing.T, handler http.Handler, method string, path string, body map[string]any) []byte {
	t.Helper()
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("%s %s status=%d body=%s", method, path, rec.Code, rec.Body.String())
	}
	return rec.Body.Bytes()
}

func mustUDPAddr(t *testing.T, value string) *net.UDPAddr {
	t.Helper()
	addr, err := net.ResolveUDPAddr("udp", value)
	if err != nil {
		t.Fatal(err)
	}
	return addr
}
