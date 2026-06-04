package biz

import (
	"net/http"
	"strings"
)

func (s *Server) internalWireRelayNodes(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Wire.RelayNodes()})
}

func (s *Server) internalWirePeerAuthz(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	view, err := s.services.Wire.PeerAuthz(r.PathValue("peerId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) internalWirePeerRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	view, err := s.services.Wire.PeerRuntimeConfig(r.PathValue("peerId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) internalWireNetworkTopology(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	view, err := s.services.Wire.NetworkTopology(r.PathValue("networkId"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) internalWireUpsertRelayNode(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	var req wireRelayNodeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	node, err := s.services.Wire.UpsertRelayNode(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) internalWireRelayNodeHeartbeat(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	var req wireNodeHeartbeatRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	node, err := s.services.Wire.RelayNodeHeartbeat(r.PathValue("nodeId"), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) internalWireRelayNodeStatus(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	var req wireNodeStatusRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	node, err := s.services.Wire.RelayNodeStatus(r.PathValue("nodeId"), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) internalWireDeleteRelayNode(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	if err := s.services.Wire.DeleteRelayNode(r.PathValue("nodeId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) internalWireDerpNodes(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Wire.DerpNodes()})
}

func (s *Server) internalWireUpsertDerpNode(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	var req wireDerpNodeRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	node, err := s.services.Wire.UpsertDerpNode(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) internalWireDerpNodeHeartbeat(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	var req wireNodeHeartbeatRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	node, err := s.services.Wire.DerpNodeHeartbeat(r.PathValue("nodeId"), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) internalWireDerpNodeStatus(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	var req wireNodeStatusRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	node, err := s.services.Wire.DerpNodeStatus(r.PathValue("nodeId"), req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) internalWireDeleteDerpNode(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	if err := s.services.Wire.DeleteDerpNode(r.PathValue("nodeId")); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) internalWireDerpMap(w http.ResponseWriter, r *http.Request) {
	if !requireInternalWireToken(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, s.services.Wire.DerpMap())
}

func (req wireRelayNodeRequest) relayNode() OpsRelayNode {
	status := "active"
	if req.Enabled != nil && !*req.Enabled {
		status = "disabled"
	}
	health := "healthy"
	if req.Healthy != nil && !*req.Healthy {
		health = "down"
	}
	return OpsRelayNode{
		NodeID:            strings.TrimSpace(req.NodeID),
		Name:              defaultString(strings.TrimSpace(req.NodeID), "Relay Node"),
		Region:            defaultString(req.RegionID, "default"),
		Transport:         "relay_udp",
		PublicAddr:        hostPort(req.Host, req.UDPPort),
		InternalAddr:      hostPort(req.Host, req.AdminPort),
		MaxBandwidthMbps:  1000,
		MonthlyTrafficGB:  10240,
		MaxSessions:       10000,
		Status:            status,
		Health:            health,
		Priority:          defaultInt(req.Priority, 100),
		TicketKeyRotation: req.TicketKeyRotation,
	}
}

func (req wireDerpNodeRequest) derpNode() OpsRelayNode {
	status := "active"
	if req.Enabled != nil && !*req.Enabled {
		status = "disabled"
	}
	health := "healthy"
	if req.Healthy != nil && !*req.Healthy {
		health = "down"
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.TrimSpace(req.NodeID)
	}
	return OpsRelayNode{
		NodeID:            strings.TrimSpace(req.NodeID),
		Name:              defaultString(name, "DERP Node"),
		Region:            defaultString(req.RegionID, "default"),
		Transport:         "derp_tcp_tls_443",
		PublicAddr:        hostPort(req.Host, req.Port),
		MaxBandwidthMbps:  1000,
		MonthlyTrafficGB:  10240,
		MaxSessions:       10000,
		Status:            status,
		Health:            health,
		Priority:          defaultInt(req.Priority, 100),
		TicketKeyRotation: req.TicketKeyRotation,
	}
}
