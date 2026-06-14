package biz

import (
	"net/http"
)

func (s *Server) listNetworks(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	if userID == "" {
		userID = r.URL.Query().Get("userId")
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Network.ListNetworks(userID)})
}

func (s *Server) createNetwork(w http.ResponseWriter, r *http.Request) {
	var req CreateNetworkRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	network, group, zone, err := s.services.Network.CreateNetwork(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"network": network, "defaultSecurityGroup": group, "defaultDNSZone": zone})
}

func (s *Server) updateNetwork(w http.ResponseWriter, r *http.Request) {
	var req UpdateNetworkRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	network, reason, err := s.services.Network.UpdateNetwork(r.PathValue("networkId"), req)
	if err != nil {
		writeError(w, err)
		return
	}
	s.notifyNetworkConfigChanged(network.NetworkID, reason, "network", "update", network.NetworkID, "")
	writeJSON(w, http.StatusOK, network)
}

func (s *Server) createDeviceInvite(w http.ResponseWriter, r *http.Request) {
	var req CreateDeviceInviteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	invite, err := s.services.Network.CreateDeviceInvite(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, invite)
}

func (s *Server) listDeviceInvites(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("userId")
	if userID == "" {
		userID = r.URL.Query().Get("userId")
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": s.services.Network.ListDeviceInvites(userID)})
}

func (s *Server) acceptDeviceInvite(w http.ResponseWriter, r *http.Request) {
	var req AcceptDeviceInviteRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	grant, invite, err := s.services.Network.AcceptDeviceInvite(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"grant": grant, "invite": invite})
}
