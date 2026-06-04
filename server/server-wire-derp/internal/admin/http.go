package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/slan/server/server-wire-derp/internal/state"
)

type Server struct {
	store   state.AdminViewStore
	service *Service
}

func New(store state.AdminViewStore) *Server {
	return &Server{store: store, service: NewService(store)}
}

func (s *Server) ensureService() {
	if s.service == nil {
		s.service = NewService(s.store)
	}
}

func (s *Server) Handler() http.Handler {
	s.ensureService()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealthz)
	mux.HandleFunc("/connections", s.handleConnections)
	mux.HandleFunc("/connections/", s.handleConnection)
	mux.HandleFunc("/sessions", s.handleSessions)
	mux.HandleFunc("/sessions/", s.handleSession)
	mux.HandleFunc("/regions", s.handleRegions)
	mux.HandleFunc("/ticket-key-status", s.handleTicketKeyStatus)
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
	writeJSON(w, http.StatusOK, map[string]any{"connections": s.service.Connections()})
}

func (s *Server) handleConnection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	peerID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/connections/"), "/")
	view, ok := s.service.Connection(peerID)
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
	writeJSON(w, http.StatusOK, map[string]any{"sessions": s.service.Sessions()})
}

func (s *Server) handleRegions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"regions": s.service.Regions()})
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	sessionID := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/sessions/"), "/")
	view, ok := s.service.Session(sessionID)
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
	writeJSON(w, http.StatusOK, s.service.Metrics())
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
