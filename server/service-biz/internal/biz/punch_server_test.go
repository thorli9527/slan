package biz

import (
	"bytes"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

func TestPunchConnectSessionHTTPProxiesAuthorizedRequest(t *testing.T) {
	var sawPunchRequest bool
	var punchRequest struct {
		NetworkID       string `json:"networkId"`
		RequesterNodeID string `json:"requesterNodeId"`
		PeerNodeID      string `json:"peerNodeId"`
		TTLSeconds      int    `json:"ttlSeconds"`
	}
	punchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawPunchRequest = true
		if r.URL.Path != "/v1/connect-sessions" {
			t.Fatalf("unexpected punch path: %s", r.URL.Path)
		}
		if got := r.Header.Get("X-Slan-Internal-Token"); got != "test-wire-token" {
			t.Fatalf("unexpected internal token: %q", got)
		}
		if got := r.Header.Get("X-Slan-Device-ID"); got != "punch-http-mac" {
			t.Fatalf("unexpected device id header: %q", got)
		}
		if got := r.Header.Get("X-Slan-MQTT-Username"); got == "" {
			t.Fatalf("expected mqtt username header")
		}
		if got := r.Header.Get("X-Slan-Punch-Signature"); got == "" {
			t.Fatalf("expected punch signature header")
		}
		if err := json.NewDecoder(r.Body).Decode(&punchRequest); err != nil {
			t.Fatalf("decode punch request: %v", err)
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"sessionId":       "pcs-test",
			"networkId":       punchRequest.NetworkID,
			"requesterNodeId": punchRequest.RequesterNodeID,
			"peerNodeId":      punchRequest.PeerNodeID,
		})
	}))
	defer punchServer.Close()

	t.Setenv("SLAN_INTERNAL_WIRE_TOKEN", "test-wire-token")
	parsedPunchURL, err := url.Parse(punchServer.URL)
	if err != nil {
		t.Fatalf("parse punch server url: %v", err)
	}
	punchHost, punchHTTPPortText, err := net.SplitHostPort(parsedPunchURL.Host)
	if err != nil {
		t.Fatalf("split punch server host: %v", err)
	}
	punchHTTPPort, err := strconv.Atoi(punchHTTPPortText)
	if err != nil {
		t.Fatalf("parse punch http port: %v", err)
	}

	server := NewServer()
	server.mqtt = MQTTConfig{Enabled: true, UsernamePrefix: "slan", PasswordSecret: "test-secret", CredentialTTLSeconds: 3600}
	if _, err := server.store.UpsertPunchNode(OpsPunchNode{
		Name:          "Test Punch",
		Region:        "test",
		PublicUDPIP:   punchHost,
		PublicUDPPort: punchHTTPPort - 1,
		Status:        "active",
		Health:        "healthy",
		Priority:      1,
	}); err != nil {
		t.Fatalf("create punch node: %v", err)
	}
	auth, network, err := server.store.RegisterUser("punch-http@example.com", "secret", "Punch HTTP")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	deviceA, deviceSession, _, err := server.store.BindDeviceSession(auth.Session.Token, "punch-http-mac", "Mac", "macos", "macOS", "15.0", "", "pub-a")
	if err != nil {
		t.Fatalf("bind requester device session: %v", err)
	}
	deviceB, _, _ := server.store.RegisterDevice(auth.User.UserID, "punch-http-android", "Android", "android", "Android", "15", "", "pub-b")
	handler := server.Routes()
	mqttCredential := deviceMQTTCredential(server.mqtt, deviceA.DeviceID, time.Now())
	headers := map[string]string{
		"X-Slan-Device-ID":        deviceA.DeviceID,
		"X-Slan-MQTT-Username":    mqttCredential.Username,
		"X-Slan-Punch-Signature":  punchMQTTSignature(deviceA.DeviceID, mqttCredential.Password),
		"X-Slan-Test-Device-Auth": deviceSession.DeviceToken,
	}

	var response map[string]any
	requestJSONWithHeaders(t, handler, http.MethodPost, "/api/networks/"+network.NetworkID+"/punch/connect-sessions", headers, map[string]any{
		"requesterNodeId": "node-" + deviceA.DeviceID,
		"peerNodeId":      "node-" + deviceB.DeviceID,
		"ttlSeconds":      30,
	}, http.StatusCreated, &response)
	if !sawPunchRequest {
		t.Fatalf("expected request to punch service")
	}
	if punchRequest.NetworkID != network.NetworkID || punchRequest.RequesterNodeID != "node-"+deviceA.DeviceID || punchRequest.PeerNodeID != "node-"+deviceB.DeviceID || punchRequest.TTLSeconds != 30 {
		t.Fatalf("unexpected punch request: %+v", punchRequest)
	}
	if response["sessionId"] != "pcs-test" {
		t.Fatalf("unexpected response: %+v", response)
	}
	if response["punchNodeId"] == "" {
		t.Fatalf("expected selected punch node id in response: %+v", response)
	}
}

func TestPunchConnectSessionHTTPRejectsUnauthorizedPeerBeforeProxy(t *testing.T) {
	punchServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("punch service should not be called for unauthorized peer")
	}))
	defer punchServer.Close()

	server := NewServer()
	auth, network, err := server.store.RegisterUser("punch-deny@example.com", "secret", "Punch Deny")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	deviceA, _, _ := server.store.RegisterDevice(auth.User.UserID, "punch-deny-mac", "Mac", "macos", "macOS", "15.0", "", "pub-a")
	handler := server.Routes()

	postJSON(t, handler, "/api/networks/"+network.NetworkID+"/punch/connect-sessions", "", map[string]any{
		"requesterNodeId": "node-" + deviceA.DeviceID,
		"peerNodeId":      "node-missing",
	}, http.StatusBadRequest, nil)
}

func TestConfiguredPunchNodesUsePublicUDPIPAndPort(t *testing.T) {
	t.Setenv("SLAN_WIRE_PUNCH_NODES", "primary=47.245.40.231:29130,backup=47.245.40.232:29130,ignored=http://47.245.40.233:29131")

	nodes := configuredPunchNodes()
	if len(nodes) != 2 {
		t.Fatalf("expected URL seed to be rejected, got %+v", nodes)
	}
	if nodes[0].Name != "primary" || nodes[0].PublicUDPIP != "47.245.40.231" || nodes[0].PublicUDPPort != 29130 {
		t.Fatalf("unexpected primary punch node: %+v", nodes[0])
	}
	if nodes[1].Name != "backup" || nodes[1].PublicUDPIP != "47.245.40.232" || nodes[1].PublicUDPPort != 29130 {
		t.Fatalf("unexpected backup punch node: %+v", nodes[1])
	}
}

func requestJSONWithHeaders(t *testing.T, handler http.Handler, method, path string, headers map[string]string, body any, want int, out any) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, path, rec.Code, want, rec.Body.String())
	}
	if out != nil {
		if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
			t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
		}
	}
}
