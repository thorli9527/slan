package biz

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInternalWireRoutesRegisterRelayAndDerpNodes(t *testing.T) {
	t.Setenv("SLAN_INTERNAL_WIRE_TOKEN", "wire-token")
	server := NewServer()
	handler := server.Routes()

	expectWireStatus(t, handler, http.MethodGet, "/internal/wire/admin/relay-nodes", "", nil, http.StatusUnauthorized)
	expectWireStatus(t, handler, http.MethodPut, "/internal/wire/admin/relay-nodes", "wire-token", map[string]any{
		"regionId": "cn-east",
		"nodeId":   "relay-a",
		"host":     "relay-a.internal",
		"udpPort":  29110,
		"healthy":  true,
	}, http.StatusOK)
	expectWireStatus(t, handler, http.MethodPost, "/internal/wire/admin/relay-nodes/cn-east/relay-a/heartbeat", "wire-token", map[string]any{
		"healthy": false,
	}, http.StatusOK)
	expectWireStatus(t, handler, http.MethodPatch, "/internal/wire/admin/relay-nodes/cn-east/relay-a/status", "wire-token", map[string]any{
		"enabled": false,
		"healthy": true,
	}, http.StatusOK)
	expectWireStatus(t, handler, http.MethodPut, "/internal/wire/admin/derp-nodes", "wire-token", map[string]any{
		"regionId": "cn-east",
		"nodeId":   "derp-a",
		"name":     "DERP A",
		"host":     "derp-a.internal",
		"port":     443,
		"healthy":  true,
	}, http.StatusOK)
	expectWireStatus(t, handler, http.MethodPatch, "/internal/wire/admin/derp-nodes/cn-east/derp-a/status", "wire-token", map[string]any{
		"enabled": true,
		"healthy": true,
	}, http.StatusOK)

	var derpMap struct {
		Regions []struct {
			RegionID string `json:"regionId"`
			Nodes    []struct {
				NodeID string `json:"nodeId"`
				Host   string `json:"host"`
				Port   int    `json:"port"`
			} `json:"nodes"`
		} `json:"regions"`
	}
	getWireJSON(t, handler, "/internal/wire/derp-map", "wire-token", &derpMap)
	if len(derpMap.Regions) != 1 || derpMap.Regions[0].Nodes[0].NodeID != "derp-a" {
		t.Fatalf("unexpected derp map: %+v", derpMap)
	}

	expectWireStatus(t, handler, http.MethodDelete, "/internal/wire/admin/relay-nodes/cn-east/relay-a", "wire-token", nil, http.StatusNoContent)
	expectWireStatus(t, handler, http.MethodDelete, "/internal/wire/admin/derp-nodes/cn-east/derp-a", "wire-token", nil, http.StatusNoContent)

	var relays struct {
		Items []struct {
			NodeID string `json:"nodeId"`
		} `json:"items"`
	}
	getWireJSON(t, handler, "/internal/wire/admin/relay-nodes", "wire-token", &relays)
	for _, relay := range relays.Items {
		if relay.NodeID == "relay-a" {
			t.Fatalf("relay-a should be deleted: %+v", relays.Items)
		}
	}
	getWireJSON(t, handler, "/internal/wire/derp-map", "wire-token", &derpMap)
	if len(derpMap.Regions) != 0 {
		t.Fatalf("deleted derp node should be absent from derp map: %+v", derpMap)
	}
}

func TestInternalWirePeerAuthzRuntimeAndTopology(t *testing.T) {
	t.Setenv("SLAN_INTERNAL_WIRE_TOKEN", "wire-token")
	server := NewServer()
	auth, network, err := server.store.RegisterUser("alice@staticlss.com", "password", "Alice")
	if err != nil {
		t.Fatalf("register user: %v", err)
	}
	device, _, err := server.store.RegisterDevice(auth.User.UserID, "mac-001", "Mac", "macos", "macOS", "15.3", "office mac", "pub")
	if err != nil {
		t.Fatalf("register device: %v", err)
	}
	if _, _, err := server.store.RenewDevice(device.DeviceID, auth.User.UserID, true, 0, 0); err != nil {
		t.Fatalf("renew device: %v", err)
	}
	handler := server.Routes()
	peerID := network.NetworkID + ":" + device.DeviceID

	var authz struct {
		NetworkID  string   `json:"networkId"`
		NodeID     string   `json:"nodeId"`
		Enabled    bool     `json:"enabled"`
		VirtualIPs []string `json:"virtualIps"`
	}
	getWireJSON(t, handler, "/internal/wire/peers/"+peerID+"/authz", "wire-token", &authz)
	if authz.NetworkID != network.NetworkID || authz.NodeID != "node-"+device.DeviceID || !authz.Enabled || len(authz.VirtualIPs) != 1 {
		t.Fatalf("unexpected peer authz: %+v", authz)
	}

	var runtime struct {
		NetworkID      string `json:"networkId"`
		NodeID         string `json:"nodeId"`
		NetworkEnabled bool   `json:"networkEnabled"`
		PreferredPath  string `json:"preferredPath"`
	}
	getWireJSON(t, handler, "/internal/wire/peers/node-"+device.DeviceID+"/runtime-config", "wire-token", &runtime)
	if runtime.NetworkID != network.NetworkID || runtime.NodeID != "node-"+device.DeviceID || !runtime.NetworkEnabled || runtime.PreferredPath != "direct_udp" {
		t.Fatalf("unexpected runtime config: %+v", runtime)
	}

	var topology struct {
		NetworkID string `json:"networkId"`
		Peers     []struct {
			PeerID     string   `json:"peerId"`
			NodeID     string   `json:"nodeId"`
			VirtualIPs []string `json:"virtualIps"`
		} `json:"peers"`
	}
	getWireJSON(t, handler, "/internal/wire/networks/"+network.NetworkID+"/topology", "wire-token", &topology)
	if topology.NetworkID != network.NetworkID || len(topology.Peers) != 1 || topology.Peers[0].PeerID != peerID {
		t.Fatalf("unexpected topology: %+v", topology)
	}
}

func expectWireStatus(t *testing.T, handler http.Handler, method, path, token string, body any, want int) {
	t.Helper()
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Slan-Internal-Token", token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, path, rec.Code, want, rec.Body.String())
	}
}

func getWireJSON(t *testing.T, handler http.Handler, path, token string, out any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("X-Slan-Internal-Token", token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s status=%d body=%s", path, rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
		t.Fatalf("decode response: %v body=%s", err, rec.Body.String())
	}
}
