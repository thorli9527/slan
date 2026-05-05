package bizclient

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/slan/server/server-wire/internal/model"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestClientEnabledRequiresBaseURL(t *testing.T) {
	if New("", "token").Enabled() {
		t.Fatal("expected empty base URL client to be disabled")
	}
	if !New(" http://127.0.0.1:28080/ ", "token").Enabled() {
		t.Fatal("expected non-empty base URL client to be enabled")
	}
}

func TestClientSendsInternalTokenAndDecodesViews(t *testing.T) {
	var paths []string
	client := New("http://server-biz.internal/", " wire-token ")
	client.http = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("X-Slan-Internal-Token") != "wire-token" {
			t.Fatalf("missing internal token header: %q", r.Header.Get("X-Slan-Internal-Token"))
		}
		paths = append(paths, r.URL.Path)
		switch r.URL.Path {
		case "/internal/wire/peers/peer-a/authz":
			return testJSONResponse(t, http.StatusOK, model.PeerAuthzView{PeerID: "peer-a", NetworkID: "net-a", Enabled: true}), nil
		case "/internal/wire/peers/peer-a/runtime-config":
			return testJSONResponse(t, http.StatusOK, model.PeerRuntimeConfigView{PeerID: "peer-a", NetworkEnabled: true}), nil
		case "/internal/wire/networks/net-a/topology":
			return testJSONResponse(t, http.StatusOK, model.NetworkTopologyView{NetworkID: "net-a"}), nil
		case "/internal/wire/derp-map":
			return testJSONResponse(t, http.StatusOK, model.DerpMap{PreferredRegionID: "cn-east", Regions: []model.DerpRegion{{RegionID: "cn-east"}}}), nil
		case "/internal/wire/admin/relay-nodes":
			return testJSONResponse(t, http.StatusOK, map[string]any{"items": []model.RelayNode{
				{RegionID: "cn-east", NodeID: "relay-a", Host: "relay-a.local", UDPPort: 29110, Enabled: true, Healthy: true},
				{RegionID: "cn-east", NodeID: "relay-disabled", Host: "relay-disabled.local", UDPPort: 29110, Enabled: false, Healthy: true},
				{RegionID: "cn-east", NodeID: "relay-stale", Host: "relay-stale.local", UDPPort: 29110, Enabled: true, Healthy: true, Stale: true},
			}}), nil
		default:
			return testStringResponse(http.StatusNotFound, "not found"), nil
		}
	})}

	authz, err := client.PeerAuthz(context.Background(), "peer-a")
	if err != nil {
		t.Fatalf("peer authz: %v", err)
	}
	if authz.PeerID != "peer-a" || authz.NetworkID != "net-a" || !authz.Enabled {
		t.Fatalf("unexpected authz: %#v", authz)
	}
	runtime, err := client.PeerRuntimeConfig(context.Background(), "peer-a")
	if err != nil {
		t.Fatalf("runtime config: %v", err)
	}
	if runtime.PeerID != "peer-a" || !runtime.NetworkEnabled {
		t.Fatalf("unexpected runtime: %#v", runtime)
	}
	topology, err := client.NetworkTopology(context.Background(), "net-a")
	if err != nil {
		t.Fatalf("network topology: %v", err)
	}
	if topology.NetworkID != "net-a" {
		t.Fatalf("unexpected topology: %#v", topology)
	}
	derpMap, err := client.DerpMap(context.Background())
	if err != nil {
		t.Fatalf("derp map: %v", err)
	}
	if derpMap.PreferredRegionID != "cn-east" {
		t.Fatalf("unexpected derp map: %#v", derpMap)
	}
	relays, err := client.RelayNodes(context.Background())
	if err != nil {
		t.Fatalf("relay nodes: %v", err)
	}
	if len(relays) != 3 || relays[0].NodeID != "relay-a" || relays[1].NodeID != "relay-disabled" || relays[2].NodeID != "relay-stale" {
		t.Fatalf("unexpected relay nodes: %#v", relays)
	}

	want := []string{
		"/internal/wire/peers/peer-a/authz",
		"/internal/wire/peers/peer-a/runtime-config",
		"/internal/wire/networks/net-a/topology",
		"/internal/wire/derp-map",
		"/internal/wire/admin/relay-nodes",
	}
	if len(paths) != len(want) {
		t.Fatalf("unexpected paths: %#v", paths)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("path[%d]=%q want %q", i, paths[i], want[i])
		}
	}
}

func TestClientReturnsErrorForNonSuccessStatus(t *testing.T) {
	client := New("http://server-biz.internal", "wire-token")
	client.http = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return testStringResponse(http.StatusForbidden, "forbidden"), nil
	})}
	if _, err := client.PeerAuthz(context.Background(), "peer-a"); err == nil {
		t.Fatal("expected non-2xx response to return error")
	}
}

func TestClientReturnsErrorForInvalidJSON(t *testing.T) {
	client := New("http://server-biz.internal", "wire-token")
	client.http = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return testStringResponse(http.StatusOK, "{"), nil
	})}
	if _, err := client.PeerAuthz(context.Background(), "peer-a"); err == nil {
		t.Fatal("expected invalid JSON response to return error")
	}
}

func testJSONResponse(t *testing.T, status int, value any) *http.Response {
	t.Helper()
	var body bytes.Buffer
	if err := json.NewEncoder(&body).Encode(value); err != nil {
		t.Fatalf("encode json: %v", err)
	}
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(body.Bytes())),
		Header:     make(http.Header),
	}
}

func testStringResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}
}
