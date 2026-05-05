package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/slan/server/server-wire/internal/model"
	"github.com/slan/server/server-wire/internal/service"
	"github.com/slan/server/server-wire/internal/store"
)

type switchingBizAuthorizer struct {
	authz model.PeerAuthzView
}

func (f *switchingBizAuthorizer) Enabled() bool { return true }

func (f *switchingBizAuthorizer) PeerAuthz(context.Context, string) (model.PeerAuthzView, error) {
	return f.authz, nil
}

func (f *switchingBizAuthorizer) PeerRuntimeConfig(context.Context, string) (model.PeerRuntimeConfigView, error) {
	return model.PeerRuntimeConfigView{NetworkEnabled: f.authz.Enabled}, nil
}

func (f *switchingBizAuthorizer) NetworkTopology(context.Context, string) (model.NetworkTopologyView, error) {
	return model.NetworkTopologyView{NetworkID: f.authz.NetworkID}, nil
}

func (f *switchingBizAuthorizer) DerpMap(context.Context) (model.DerpMap, error) {
	return model.DerpMap{}, nil
}

func (f *switchingBizAuthorizer) RelayNodes(context.Context) ([]model.RelayNode, error) {
	return nil, nil
}

func TestPeerReadRoutes(t *testing.T) {
	svc := service.New(store.NewMemoryStore())
	registerBody := mustJSON(t, model.RegisterPeerRequest{
		Peer: model.PeerRegistration{
			PeerID:            "peer-http",
			NetworkID:         "net-http",
			NodeID:            "node-http",
			SupportsDirectUDP: true,
			SupportsRelayUDP:  true,
			VirtualIPs:        []string{"100.64.0.2"},
			AllowedIPs:        []string{"10.0.0.0/24"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/peers/register", bytes.NewReader(registerBody))
	rec := httptest.NewRecorder()
	handleRegisterPeer(svc)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("register status=%d body=%s", rec.Code, rec.Body.String())
	}

	healthBody := mustJSON(t, model.ReportPathHealthRequest{
		PeerID: "peer-http",
		Probes: []model.PathProbe{{Path: model.PathDirectUDP, Reachable: true, RTTMs: 11}},
	})
	req = httptest.NewRequest(http.MethodPost, "/v1/peers/path-health", bytes.NewReader(healthBody))
	rec = httptest.NewRecorder()
	handleReportPathHealth(svc)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("path-health status=%d body=%s", rec.Code, rec.Body.String())
	}

	derpHealthBody := mustJSON(t, model.ReportDerpHealthRequest{
		PeerID:  "peer-http",
		Samples: []model.DerpHealthSample{{RegionID: "cn-east", NodeID: "derp-cn-east-1", Reachable: true, RTTMs: 40}},
	})
	req = httptest.NewRequest(http.MethodPost, "/v1/peers/derp-health", bytes.NewReader(derpHealthBody))
	rec = httptest.NewRecorder()
	handleReportDerpHealth(svc)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("derp-health status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/peers/peer-http", nil)
	rec = httptest.NewRecorder()
	handleGetPeerRoutes(svc)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get peer status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/peers/peer-http/runtime-config", nil)
	rec = httptest.NewRecorder()
	handleGetPeerRoutes(svc)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("runtime config status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/internal/wire/peers/peer-http/authz", nil)
	rec = httptest.NewRecorder()
	handleInternalPeerRoutes(svc)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authz status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/internal/wire/networks/net-http/topology", nil)
	rec = httptest.NewRecorder()
	handleInternalNetworkRoutes(svc)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("topology status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/v1/derp/map", nil)
	rec = httptest.NewRecorder()
	handleGetDerpMap(svc)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("derp map status=%d body=%s", rec.Code, rec.Body.String())
	}

	derpTicketBody := mustJSON(t, model.IssueDerpTicketRequest{PeerID: "peer-http"})
	req = httptest.NewRequest(http.MethodPost, "/v1/derp/tickets", bytes.NewReader(derpTicketBody))
	rec = httptest.NewRecorder()
	handleIssueDerpTicket(svc)(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("derp ticket status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHTTPRoutesRejectOperationsAfterBizDisable(t *testing.T) {
	biz := &switchingBizAuthorizer{
		authz: model.PeerAuthzView{
			PeerID:     "peer-disabled-http",
			NetworkID:  "net-disabled-http",
			NodeID:     "node-disabled-http",
			Enabled:    true,
			VirtualIPs: []string{"100.64.10.40"},
			AllowedIPs: []string{"100.64.10.40/32"},
		},
	}
	mux := newMux(service.NewWithBiz(store.NewMemoryStore(), biz))

	expectStatus(t, mux, http.MethodPost, "/v1/peers/register", model.RegisterPeerRequest{
		Peer: model.PeerRegistration{
			PeerID:                "peer-disabled-http",
			NetworkID:             "client-forged-net",
			NodeID:                "client-forged-node",
			SupportsRelayUDP:      true,
			SupportsDerpTCPTLS443: true,
		},
	}, http.StatusOK)

	biz.authz.Enabled = false

	expectStatus(t, mux, http.MethodPost, "/v1/peers/endpoints", model.UpdateEndpointsRequest{
		PeerID:    "peer-disabled-http",
		Endpoints: []model.Endpoint{{Kind: "udp", Address: "192.0.2.20", Port: 51820}},
	}, http.StatusBadRequest)
	expectStatus(t, mux, http.MethodPost, "/v1/peers/path-health", model.ReportPathHealthRequest{
		PeerID: "peer-disabled-http",
		Probes: []model.PathProbe{{Path: model.PathRelayUDP, Reachable: true}},
	}, http.StatusBadRequest)
	expectStatus(t, mux, http.MethodPost, "/v1/peers/derp-health", model.ReportDerpHealthRequest{
		PeerID:  "peer-disabled-http",
		Samples: []model.DerpHealthSample{{RegionID: "cn-east", NodeID: "derp-cn-east-1", Reachable: true}},
	}, http.StatusBadRequest)
	expectStatus(t, mux, http.MethodPost, "/v1/peers/active-path", model.UpdateActivePathRequest{
		PeerID: "peer-disabled-http",
		Path:   model.PathRelayUDP,
	}, http.StatusBadRequest)
	expectStatus(t, mux, http.MethodPost, "/v1/relay/tickets", model.IssueRelayTicketRequest{
		PeerID: "peer-disabled-http",
	}, http.StatusBadRequest)
	expectStatus(t, mux, http.MethodPost, "/v1/derp/tickets", model.IssueDerpTicketRequest{
		PeerID: "peer-disabled-http",
	}, http.StatusBadRequest)
	expectStatus(t, mux, http.MethodPost, "/v1/path-plan", model.PathPlanRequest{
		PeerID: "peer-disabled-http",
	}, http.StatusBadRequest)
	expectStatus(t, mux, http.MethodGet, "/v1/peers/peer-disabled-http", nil, http.StatusBadRequest)
	expectStatus(t, mux, http.MethodGet, "/v1/peers/peer-disabled-http/runtime-config", nil, http.StatusBadRequest)
	expectStatus(t, mux, http.MethodGet, "/internal/wire/peers/peer-disabled-http/authz", nil, http.StatusBadRequest)
	expectStatus(t, mux, http.MethodGet, "/internal/wire/peers/peer-disabled-http/runtime-config", nil, http.StatusBadRequest)
}

func TestHTTPRegisterRejectsUnauthorizedBizPeer(t *testing.T) {
	biz := &switchingBizAuthorizer{
		authz: model.PeerAuthzView{
			PeerID:  "peer-other",
			Enabled: true,
		},
	}
	mux := newMux(service.NewWithBiz(store.NewMemoryStore(), biz))

	expectStatus(t, mux, http.MethodPost, "/v1/peers/register", model.RegisterPeerRequest{
		Peer: model.PeerRegistration{PeerID: "peer-client"},
	}, http.StatusBadRequest)
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return payload
}

func expectStatus(t *testing.T, handler http.Handler, method, target string, body any, want int) {
	t.Helper()
	expectStatusWithToken(t, handler, method, target, "", body, want)
}

func expectStatusWithToken(t *testing.T, handler http.Handler, method, target, token string, body any, want int) {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(mustJSON(t, body))
	}
	req := httptest.NewRequest(method, target, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("X-Slan-Internal-Token", token)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, target, rec.Code, want, rec.Body.String())
	}
}
