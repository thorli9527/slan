package biz

import (
	"net/http"
	"os"
	"strings"
)

func (s *Server) registerInternalWireRoutes(mux *http.ServeMux) {
	registerAPIRoutes(mux, []apiRoute{
		route(http.MethodGet, "/internal/wire/peers/{peerId}/authz", s.internalWirePeerAuthz),
		route(http.MethodGet, "/internal/wire/peers/{peerId}/runtime-config", s.internalWirePeerRuntimeConfig),
		route(http.MethodGet, "/internal/wire/networks/{networkId}/topology", s.internalWireNetworkTopology),
		route(http.MethodGet, "/internal/wire/admin/relay-nodes", s.internalWireRelayNodes),
		route(http.MethodPut, "/internal/wire/admin/relay-nodes", s.internalWireUpsertRelayNode),
		route(http.MethodPost, "/internal/wire/admin/relay-nodes/{regionId}/{nodeId}/heartbeat", s.internalWireRelayNodeHeartbeat),
		route(http.MethodPatch, "/internal/wire/admin/relay-nodes/{regionId}/{nodeId}/status", s.internalWireRelayNodeStatus),
		route(http.MethodDelete, "/internal/wire/admin/relay-nodes/{regionId}/{nodeId}", s.internalWireDeleteRelayNode),
		route(http.MethodGet, "/internal/wire/admin/derp-nodes", s.internalWireDerpNodes),
		route(http.MethodPut, "/internal/wire/admin/derp-nodes", s.internalWireUpsertDerpNode),
		route(http.MethodPost, "/internal/wire/admin/derp-nodes/{regionId}/{nodeId}/heartbeat", s.internalWireDerpNodeHeartbeat),
		route(http.MethodPatch, "/internal/wire/admin/derp-nodes/{regionId}/{nodeId}/status", s.internalWireDerpNodeStatus),
		route(http.MethodDelete, "/internal/wire/admin/derp-nodes/{regionId}/{nodeId}", s.internalWireDeleteDerpNode),
		route(http.MethodGet, "/internal/wire/derp-map", s.internalWireDerpMap),
	})
}

func requireInternalWireToken(w http.ResponseWriter, r *http.Request) bool {
	expected := strings.TrimSpace(os.Getenv("SLAN_INTERNAL_WIRE_TOKEN"))
	if expected == "" || strings.TrimSpace(r.Header.Get("X-Slan-Internal-Token")) != expected {
		writeError(w, errUnauthorized)
		return false
	}
	return true
}
