package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/service"
)

type fakeWireService struct {
	authzErr    error
	runtimeErr  error
	topologyErr error
	derpNodes   map[string]dto.WireDerpNodeRecord
	relayNodes  map[string]dto.WireRelayNodeRecord
}

func (f fakeWireService) PeerAuthz(peerID string) (dto.WirePeerAuthzView, error) {
	if f.authzErr != nil {
		return dto.WirePeerAuthzView{}, f.authzErr
	}
	return dto.WirePeerAuthzView{
		PeerID:     peerID,
		NodeID:     peerID,
		NetworkID:  "net-wire-test",
		Enabled:    true,
		VirtualIPs: []string{"100.64.20.10"},
		AllowedIPs: []string{"100.64.20.10/32"},
	}, nil
}

func (f fakeWireService) PeerRuntimeConfig(peerID string) (dto.WirePeerRuntimeConfigView, error) {
	if f.runtimeErr != nil {
		return dto.WirePeerRuntimeConfigView{}, f.runtimeErr
	}
	return dto.WirePeerRuntimeConfigView{
		PeerID:         peerID,
		NodeID:         peerID,
		NetworkID:      "net-wire-test",
		NetworkEnabled: true,
		VirtualIPs:     []string{"100.64.20.10"},
		AllowedIPs:     []string{"100.64.20.10/32"},
	}, nil
}

func (f fakeWireService) NetworkTopology(networkID string) (dto.WireNetworkTopologyView, error) {
	if f.topologyErr != nil {
		return dto.WireNetworkTopologyView{}, f.topologyErr
	}
	return dto.WireNetworkTopologyView{NetworkID: networkID}, nil
}

func (f fakeWireService) DerpMap() (dto.WireDerpMapView, error) {
	if len(f.derpNodes) > 0 {
		out := dto.WireDerpMapView{}
		regionIndex := map[string]int{}
		for _, node := range f.derpNodes {
			if !node.Enabled || !node.Healthy {
				continue
			}
			idx, ok := regionIndex[node.RegionID]
			if !ok {
				if out.PreferredRegionID == "" {
					out.PreferredRegionID = node.RegionID
				}
				out.Regions = append(out.Regions, dto.WireDerpRegion{RegionID: node.RegionID, Name: node.Name})
				idx = len(out.Regions) - 1
				regionIndex[node.RegionID] = idx
			}
			out.Regions[idx].Nodes = append(out.Regions[idx].Nodes, dto.WireDerpNode{
				RegionID: node.RegionID,
				NodeID:   node.NodeID,
				Host:     node.Host,
				Port:     node.Port,
			})
		}
		return out, nil
	}
	return dto.WireDerpMapView{Regions: []dto.WireDerpRegion{{RegionID: "cn-east"}}}, nil
}

func (f fakeWireService) ListDerpNodes() ([]dto.WireDerpNodeRecord, error) {
	if len(f.derpNodes) > 0 {
		out := make([]dto.WireDerpNodeRecord, 0, len(f.derpNodes))
		for _, node := range f.derpNodes {
			out = append(out, node)
		}
		return out, nil
	}
	return []dto.WireDerpNodeRecord{{RegionID: "cn-east", NodeID: "derp-a", Healthy: true}}, nil
}

func (f fakeWireService) UpsertDerpNode(req dto.UpsertWireDerpNodeRequest) (dto.WireDerpNodeRecord, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	healthy := true
	if req.Healthy != nil {
		healthy = *req.Healthy
	}
	record := dto.WireDerpNodeRecord{RegionID: req.RegionID, NodeID: req.NodeID, Name: req.Name, Host: req.Host, Port: req.Port, Enabled: enabled, Healthy: healthy, Priority: req.Priority}
	if f.derpNodes != nil {
		f.derpNodes[req.RegionID+"/"+req.NodeID] = record
	}
	return record, nil
}

func (f fakeWireService) UpdateDerpNodeHealth(regionID, nodeID string, req dto.WireNodeHeartbeatRequest) (dto.WireDerpNodeRecord, error) {
	record := dto.WireDerpNodeRecord{RegionID: regionID, NodeID: nodeID, Enabled: true, Healthy: req.Healthy}
	if f.derpNodes != nil {
		key := regionID + "/" + nodeID
		record = f.derpNodes[key]
		record.Healthy = req.Healthy
		f.derpNodes[key] = record
	}
	return record, nil
}

func (f fakeWireService) UpdateDerpNodeStatus(regionID, nodeID string, req dto.UpdateWireNodeStatusRequest) (dto.WireDerpNodeRecord, error) {
	record := dto.WireDerpNodeRecord{RegionID: regionID, NodeID: nodeID}
	if f.derpNodes != nil {
		key := regionID + "/" + nodeID
		record = f.derpNodes[key]
		if req.Enabled != nil {
			record.Enabled = *req.Enabled
		}
		if req.Healthy != nil {
			record.Healthy = *req.Healthy
		}
		f.derpNodes[key] = record
	}
	return record, nil
}

func (f fakeWireService) ListRelayNodes() ([]dto.WireRelayNodeRecord, error) {
	if len(f.relayNodes) > 0 {
		out := make([]dto.WireRelayNodeRecord, 0, len(f.relayNodes))
		for _, node := range f.relayNodes {
			out = append(out, node)
		}
		return out, nil
	}
	return []dto.WireRelayNodeRecord{{RegionID: "cn-east", NodeID: "relay-a", Healthy: true}}, nil
}

func (f fakeWireService) UpsertRelayNode(req dto.UpsertWireRelayNodeRequest) (dto.WireRelayNodeRecord, error) {
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	healthy := true
	if req.Healthy != nil {
		healthy = *req.Healthy
	}
	record := dto.WireRelayNodeRecord{RegionID: req.RegionID, NodeID: req.NodeID, Host: req.Host, UDPPort: req.UDPPort, AdminPort: req.AdminPort, Enabled: enabled, Healthy: healthy, Priority: req.Priority}
	if f.relayNodes != nil {
		f.relayNodes[req.RegionID+"/"+req.NodeID] = record
	}
	return record, nil
}

func (f fakeWireService) UpdateRelayNodeHealth(regionID, nodeID string, req dto.WireNodeHeartbeatRequest) (dto.WireRelayNodeRecord, error) {
	record := dto.WireRelayNodeRecord{RegionID: regionID, NodeID: nodeID, Enabled: true, Healthy: req.Healthy}
	if f.relayNodes != nil {
		key := regionID + "/" + nodeID
		record = f.relayNodes[key]
		record.Healthy = req.Healthy
		f.relayNodes[key] = record
	}
	return record, nil
}

func (f fakeWireService) UpdateRelayNodeStatus(regionID, nodeID string, req dto.UpdateWireNodeStatusRequest) (dto.WireRelayNodeRecord, error) {
	record := dto.WireRelayNodeRecord{RegionID: regionID, NodeID: nodeID}
	if f.relayNodes != nil {
		key := regionID + "/" + nodeID
		record = f.relayNodes[key]
		if req.Enabled != nil {
			record.Enabled = *req.Enabled
		}
		if req.Healthy != nil {
			record.Healthy = *req.Healthy
		}
		f.relayNodes[key] = record
	}
	return record, nil
}

func TestInternalWireRoutesRequireToken(t *testing.T) {
	router := internalWireTestRouter(t, "wire-token", fakeWireService{})

	expectInternalWireStatus(t, router, "/internal/wire/peers/peer-a/authz", "", http.StatusUnauthorized)
	expectInternalWireStatus(t, router, "/internal/wire/peers/peer-a/authz", "wrong-token", http.StatusUnauthorized)
	expectInternalWireStatus(t, router, "/internal/wire/peers/peer-a/authz", "wire-token", http.StatusOK)
}

func TestInternalWireRoutesRejectEmptyConfiguredToken(t *testing.T) {
	router := internalWireTestRouter(t, "", fakeWireService{})

	expectInternalWireStatus(t, router, "/internal/wire/peers/peer-a/authz", "", http.StatusUnauthorized)
	expectInternalWireStatus(t, router, "/internal/wire/peers/peer-a/authz", "any-token", http.StatusUnauthorized)
}

func TestInternalWireRoutesPropagateAuthorizationErrors(t *testing.T) {
	router := internalWireTestRouter(t, "wire-token", fakeWireService{
		authzErr:    service.ErrForbidden,
		runtimeErr:  service.ErrForbidden,
		topologyErr: service.ErrNotFound,
	})

	expectInternalWireStatus(t, router, "/internal/wire/peers/peer-disabled/authz", "wire-token", http.StatusForbidden)
	expectInternalWireStatus(t, router, "/internal/wire/peers/peer-disabled/runtime-config", "wire-token", http.StatusForbidden)
	expectInternalWireStatus(t, router, "/internal/wire/networks/missing-net/topology", "wire-token", http.StatusNotFound)
}

func TestInternalWireAdminNodeRoutes(t *testing.T) {
	router := internalWireTestRouter(t, "wire-token", fakeWireService{})

	expectInternalWireStatus(t, router, "/internal/wire/derp-map", "wire-token", http.StatusOK)
	expectInternalWireStatus(t, router, "/internal/wire/admin/derp-nodes", "wire-token", http.StatusOK)
	expectInternalWireStatus(t, router, "/internal/wire/admin/relay-nodes", "wire-token", http.StatusOK)
}

func TestInternalWireAdminNodeManagementBehavior(t *testing.T) {
	enabled := true
	disabled := false
	router := internalWireTestRouter(t, "wire-token", fakeWireService{
		derpNodes:  map[string]dto.WireDerpNodeRecord{},
		relayNodes: map[string]dto.WireRelayNodeRecord{},
	})

	expectInternalWireJSONStatus(t, router, http.MethodPut, "/internal/wire/admin/derp-nodes", "wire-token", dto.UpsertWireDerpNodeRequest{
		RegionID: "cn-east", NodeID: "derp-a", Host: "derp-a.local", Port: 443, Enabled: &enabled,
	}, http.StatusOK)
	expectInternalWireJSONStatus(t, router, http.MethodPut, "/internal/wire/admin/derp-nodes", "wire-token", dto.UpsertWireDerpNodeRequest{
		RegionID: "cn-east", NodeID: "derp-b", Host: "derp-b.local", Port: 443, Enabled: &disabled,
	}, http.StatusOK)
	expectInternalWireJSONStatus(t, router, http.MethodPost, "/internal/wire/admin/derp-nodes/cn-east/derp-a/heartbeat", "wire-token", dto.WireNodeHeartbeatRequest{Healthy: true}, http.StatusOK)
	expectInternalWireJSONStatus(t, router, http.MethodPut, "/internal/wire/admin/relay-nodes", "wire-token", dto.UpsertWireRelayNodeRequest{
		RegionID: "cn-east", NodeID: "relay-a", Host: "relay-a.local", UDPPort: 29110, Enabled: &enabled,
	}, http.StatusOK)
	expectInternalWireJSONStatus(t, router, http.MethodPost, "/internal/wire/admin/relay-nodes/cn-east/relay-a/heartbeat", "wire-token", dto.WireNodeHeartbeatRequest{Healthy: false}, http.StatusOK)
	expectInternalWireJSONStatus(t, router, http.MethodPatch, "/internal/wire/admin/derp-nodes/cn-east/derp-a/status", "wire-token", dto.UpdateWireNodeStatusRequest{Enabled: &disabled}, http.StatusOK)
	expectInternalWireJSONStatus(t, router, http.MethodPatch, "/internal/wire/admin/relay-nodes/cn-east/relay-a/status", "wire-token", dto.UpdateWireNodeStatusRequest{Enabled: &disabled, Healthy: &disabled}, http.StatusOK)

	var derpMap dto.WireDerpMapView
	getInternalWireJSON(t, router, "/internal/wire/derp-map", "wire-token", &derpMap)
	if len(derpMap.Regions) != 0 {
		t.Fatalf("expected disabled derp nodes to be removed from map, got %#v", derpMap)
	}
	expectInternalWireJSONStatus(t, router, http.MethodPut, "/internal/wire/admin/derp-nodes", "", dto.UpsertWireDerpNodeRequest{
		RegionID: "cn-east", NodeID: "derp-c", Host: "derp-c.local",
	}, http.StatusUnauthorized)
}

func internalWireTestRouter(t *testing.T, token string, wire service.Wire) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	registerInternalWireRoutes(router.Group(""), routerDeps{
		Config: configs.Config{Internal: configs.InternalConfig{WireToken: token}},
		Wire:   wire,
	})
	return router
}

func expectInternalWireStatus(t *testing.T, router http.Handler, path, token string, want int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("X-Slan-Internal-Token", token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("GET %s status=%d want=%d body=%s", path, rec.Code, want, rec.Body.String())
	}
}

func expectInternalWireJSONStatus(t *testing.T, router http.Handler, method, path, token string, body any, want int) {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Slan-Internal-Token", token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != want {
		t.Fatalf("%s %s status=%d want=%d body=%s", method, path, rec.Code, want, rec.Body.String())
	}
}

func getInternalWireJSON(t *testing.T, router http.Handler, path, token string, out any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("X-Slan-Internal-Token", token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s status=%d body=%s", path, rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
		t.Fatal(err)
	}
}
