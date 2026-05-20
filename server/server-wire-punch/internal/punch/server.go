package punch

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/slan/server/server-wire-punch/internal/config"
)

type Server struct {
	conn  *net.UDPConn
	store *Store
	cfg   config.Config
}

type UDPMessage struct {
	Kind      string `json:"kind"`
	NetworkID string `json:"networkId"`
	NodeID    string `json:"nodeId"`
	Type      string `json:"type,omitempty"`
	Address   string `json:"address,omitempty"`
	NATType   string `json:"natType,omitempty"`
}

func NewServer(cfg config.Config) (*Server, error) {
	addr, err := net.ResolveUDPAddr("udp", cfg.ListenAddr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	return &Server{
		conn:  conn,
		store: NewStore(),
		cfg:   cfg,
	}, nil
}

func (s *Server) Serve() error {
	defer s.conn.Close()
	go func() {
		httpServer := &http.Server{
			Addr:    s.cfg.HTTPListenAddr,
			Handler: s.Handler(),
		}
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("punch http server stopped: %v", err)
		}
	}()
	log.Printf("wire punch udp listening on %s http=%s", s.cfg.ListenAddr, s.cfg.HTTPListenAddr)
	buf := make([]byte, 4096)
	for {
		n, addr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			return err
		}
		if err := s.handlePacket(addr, buf[:n]); err != nil {
			_ = s.write(addr, map[string]any{
				"kind":    "error",
				"message": err.Error(),
			})
		}
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "server-wire-punch"})
	})
	mux.HandleFunc("/v1/stats", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, s.store.Stats())
	})
	mux.HandleFunc("/v1/endpoints", s.handleEndpointCollection)
	mux.HandleFunc("/v1/endpoints/", s.handleEndpointItem)
	mux.HandleFunc("/v1/connect-sessions", s.handleConnectSessions)
	mux.HandleFunc("/v1/connect-sessions/", s.handleConnectSessionItem)
	return mux
}

func (s *Server) handlePacket(addr *net.UDPAddr, payload []byte) error {
	var msg UDPMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode punch packet: %w", err)
	}
	switch strings.TrimSpace(msg.Kind) {
	case "ping":
		return s.write(addr, map[string]any{"kind": "pong"})
	case "endpoint_probe", "endpoint_report":
		endpoint, err := s.upsertEndpoint(msg.NetworkID, msg.NodeID, msg.Type, msg.Address, msg.NATType, addr.String(), "")
		if err != nil {
			return err
		}
		return s.write(addr, map[string]any{
			"kind":     "endpoint_reflexive",
			"endpoint": endpoint,
		})
	default:
		return fmt.Errorf("unsupported punch packet kind %q", msg.Kind)
	}
}

func (s *Server) handleEndpointCollection(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if !s.authorized(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		if !s.authorizedDevice(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "device_token_required"})
			return
		}
		var req struct {
			NetworkID string `json:"networkId"`
			NodeID    string `json:"nodeId"`
			Type      string `json:"type"`
			Address   string `json:"address"`
			NATType   string `json:"natType"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
			return
		}
		endpoint, err := s.upsertEndpoint(req.NetworkID, req.NodeID, req.Type, req.Address, req.NATType, remoteAddress(r), r.UserAgent())
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, endpoint)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleEndpointItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/endpoints/"), "/")
	if len(parts) != 2 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	endpoint, ok := s.store.Endpoint(parts[0], parts[1])
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "endpoint_not_found"})
		return
	}
	writeJSON(w, http.StatusOK, endpoint)
}

func (s *Server) handleConnectSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if !s.authorized(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if !s.authorizedDevice(r) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "device_token_required"})
		return
	}
	var req struct {
		NetworkID       string `json:"networkId"`
		RequesterNodeID string `json:"requesterNodeId"`
		PeerNodeID      string `json:"peerNodeId"`
		TTLSeconds      int    `json:"ttlSeconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_json"})
		return
	}
	if strings.TrimSpace(req.NetworkID) == "" || strings.TrimSpace(req.RequesterNodeID) == "" || strings.TrimSpace(req.PeerNodeID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "networkId requesterNodeId peerNodeId are required"})
		return
	}
	ttl := s.cfg.SessionTTL
	if req.TTLSeconds > 0 {
		ttl = time.Duration(req.TTLSeconds) * time.Second
	}
	session := s.store.CreateSession(req.NetworkID, req.RequesterNodeID, req.PeerNodeID, ttl)
	s.notifyConnectSession(session)
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleConnectSessionItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	sessionID := strings.TrimPrefix(r.URL.Path, "/v1/connect-sessions/")
	session, ok := s.store.Session(sessionID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session_not_found"})
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) upsertEndpoint(networkID, nodeID, endpointType, address, natType, observedFrom, userAgent string) (Endpoint, error) {
	networkID = strings.TrimSpace(networkID)
	nodeID = strings.TrimSpace(nodeID)
	if networkID == "" || nodeID == "" {
		return Endpoint{}, errors.New("networkId and nodeId are required")
	}
	now := time.Now()
	reflexive := strings.TrimSpace(observedFrom)
	if reflexive == "" {
		reflexive = strings.TrimSpace(address)
	}
	endpoint := Endpoint{
		NetworkID:    networkID,
		NodeID:       nodeID,
		Type:         defaultString(endpointType, "reflexive"),
		Address:      strings.TrimSpace(address),
		Reflexive:    reflexive,
		NATType:      defaultString(natType, "unknown"),
		UpdatedAt:    now,
		ExpiresAt:    now.Add(s.cfg.EndpointTTL),
		UserAgent:    userAgent,
		ObservedFrom: strings.TrimSpace(observedFrom),
	}
	if endpoint.Address == "" {
		endpoint.Address = endpoint.Reflexive
	}
	return s.store.PutEndpoint(endpoint), nil
}

func (s *Server) write(addr *net.UDPAddr, body map[string]any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	_, err = s.conn.WriteToUDP(payload, addr)
	return err
}

func (s *Server) notifyConnectSession(session ConnectSession) {
	if s.conn == nil {
		return
	}
	if session.Requester != nil && session.Peer != nil {
		s.notifyEndpoint(session.Requester, map[string]any{
			"kind":      "connect_session",
			"sessionId": session.SessionID,
			"networkId": session.NetworkID,
			"nodeId":    session.RequesterNodeID,
			"peer":      session.Peer,
			"expiresAt": session.ExpiresAt,
		})
		s.notifyEndpoint(session.Peer, map[string]any{
			"kind":      "connect_session",
			"sessionId": session.SessionID,
			"networkId": session.NetworkID,
			"nodeId":    session.PeerNodeID,
			"peer":      session.Requester,
			"expiresAt": session.ExpiresAt,
		})
	}
}

func (s *Server) notifyEndpoint(endpoint *Endpoint, body map[string]any) {
	if endpoint == nil {
		return
	}
	address := strings.TrimSpace(endpoint.ObservedFrom)
	if address == "" {
		address = strings.TrimSpace(endpoint.Reflexive)
	}
	if address == "" {
		address = strings.TrimSpace(endpoint.Address)
	}
	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return
	}
	_ = s.write(udpAddr, body)
}

func (s *Server) authorized(r *http.Request) bool {
	token := strings.TrimSpace(s.cfg.InternalWireToken)
	if token == "" || strings.Contains(strings.ToLower(token), "change-me") {
		return true
	}
	got := strings.TrimSpace(r.Header.Get("X-Slan-Internal-Token"))
	return got == token
}

func (s *Server) authorizedDevice(r *http.Request) bool {
	token := strings.TrimSpace(s.cfg.InternalWireToken)
	if token == "" || strings.Contains(strings.ToLower(token), "change-me") {
		return true
	}
	return strings.TrimSpace(r.Header.Get("X-Slan-Device-ID")) != "" &&
		strings.TrimSpace(r.Header.Get("X-Slan-MQTT-Username")) != "" &&
		strings.TrimSpace(r.Header.Get("X-Slan-Punch-Signature")) != ""
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func remoteAddress(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); value != "" {
		return strings.TrimSpace(strings.Split(value, ",")[0])
	}
	return r.RemoteAddr
}

func defaultString(value, fallback string) string {
	if value = strings.TrimSpace(value); value != "" {
		return value
	}
	return fallback
}
