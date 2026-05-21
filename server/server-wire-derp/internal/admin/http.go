package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/slan/server/server-wire-derp/internal/state"
)

type Server struct {
	store state.AdminViewStore
}

func New(store state.AdminViewStore) *Server {
	return &Server{store: store}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/v1/connections", s.handleConnections)
	mux.HandleFunc("/v1/connections/", s.handleConnection)
	mux.HandleFunc("/v1/sessions", s.handleSessions)
	mux.HandleFunc("/v1/sessions/", s.handleSession)
	mux.HandleFunc("/v1/regions", s.handleRegions)
	mux.HandleFunc("/v1/ticket-key-status", s.handleTicketKeyStatus)
	mux.HandleFunc("/metrics", s.handleMetrics)
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "server-wire-derp"})
}

func (s *Server) handleConnections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connections": s.store.Connections()})
}

func (s *Server) handleConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	peerID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/connections/"), "/")
	view, ok := s.store.Connection(peerID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"code": "connection_not_found", "message": "connection not found"})
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sessions": s.store.Sessions()})
}

func (s *Server) handleRegions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"regions": s.store.Regions()})
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	sessionID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/sessions/"), "/")
	view, ok := s.store.Session(sessionID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"code": "session_not_found", "message": "session not found"})
		return
	}
	writeJSON(w, http.StatusOK, view)
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, s.store.Metrics())
}

func (s *Server) handleTicketKeyStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, state.CurrentTicketKeyStatus())
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
