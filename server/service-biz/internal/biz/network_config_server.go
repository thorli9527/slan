package biz

import (
	"net/http"
	"strings"
)

func (s *Server) networkConfig(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(r.URL.Query().Get("deviceId"))
	if deviceID == "" {
		writeError(w, errBadRequest)
		return
	}
	config, err := s.services.Network.NetworkConfig(r.PathValue("networkId"), deviceID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, config)
}

func (s *Server) relayCandidates(w http.ResponseWriter, r *http.Request) {
	deviceID := strings.TrimSpace(r.URL.Query().Get("deviceId"))
	if deviceID == "" && r.Method == http.MethodPost {
		var req RelayCandidatesRequest
		if !decodeJSON(w, r, &req) {
			return
		}
		deviceID = strings.TrimSpace(req.DeviceID)
	}
	if deviceID == "" {
		writeError(w, errBadRequest)
		return
	}
	candidates, err := s.services.Network.RelayCandidates(r.PathValue("networkId"), deviceID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": candidates})
}

func (s *Server) issueRelayTicket(w http.ResponseWriter, r *http.Request) {
	var req IssueRelayTicketRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	ticket, err := s.services.Network.IssueRelayTicket(req)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, ticket)
}
