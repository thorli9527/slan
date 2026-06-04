package admin

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/slan/server/server-wire-relay/internal/state"
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
	mux.HandleFunc("/sessions", s.handleSessions)
	mux.HandleFunc("/sessions/", s.handleSession)
	mux.HandleFunc("/ticket-key-status", s.handleTicketKeyStatus)
	mux.HandleFunc("/metrics", s.handleMetrics)
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"service": "server-wire-relay",
	})
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sessions": s.service.Sessions(),
	})
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	sessionID := strings.TrimPrefix(r.URL.Path, "/sessions/")
	sessionID = strings.TrimSuffix(sessionID, "/")
	if sessionID == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	session, ok := s.service.Session(sessionID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{
			"code":    "session_not_found",
			"message": "session not found",
		})
		return
	}
	writeJSON(w, http.StatusOK, session)
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
