package punch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/slan/server/server-wire-punch/internal/bizclient"
	"github.com/slan/server/server-wire-punch/internal/config"
)

// Server 同时承载 punch UDP 探测入口和 HTTP 管理/协商入口。
type Server struct {
	conn    *net.UDPConn
	store   Repository
	service *Service
	cfg     config.Config
}

// UDPMessage 是 punch UDP 数据面使用的轻量消息格式。
type UDPMessage struct {
	// Kind 表示消息类型，例如 ping、endpoint_probe、endpoint_report。
	Kind string `json:"kind"`
	// NetworkID 是节点所属虚拟网络。
	NetworkID string `json:"networkId"`
	// NodeID 是上报端点的节点 ID。
	NodeID string `json:"nodeId"`
	// Type 是端点类别。
	Type string `json:"type,omitempty"`
	// Address 是客户端主动上报的候选地址。
	Address string `json:"address,omitempty"`
	// NATType 是客户端识别到的 NAT 类型。
	NATType string `json:"natType,omitempty"`
}

// NewServer 使用默认内存仓储创建 punch 服务。
func NewServer(cfg config.Config) (*Server, error) {
	return NewServerWithRepository(cfg, NewStore())
}

// NewServerWithRepository 使用调用方提供的仓储创建 punch 服务，便于测试或替换实现。
func NewServerWithRepository(cfg config.Config, store Repository) (*Server, error) {
	addr, err := net.ResolveUDPAddr("udp", cfg.ListenAddr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	if store == nil {
		store = NewStore()
	}
	return &Server{
		conn:    conn,
		store:   store,
		service: NewService(store, cfg),
		cfg:     cfg,
	}, nil
}

func (s *Server) ensureService() {
	if s.service == nil {
		s.service = NewService(s.store, s.cfg)
	}
}

// Serve 启动 HTTP 协商入口，并在当前 goroutine 中处理 UDP 探测包。
func (s *Server) Serve() error {
	defer s.conn.Close()
	go s.registerAndHeartbeat()
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

func (s *Server) registerAndHeartbeat() {
	client := bizclient.New(s.cfg.BizURL, s.cfg.InternalWireToken)
	if !client.Enabled() {
		log.Printf("wire punch biz registration disabled: missing SLAN_BIZ_URL or SLAN_INTERNAL_WIRE_TOKEN")
		return
	}
	enabled, healthy := s.cfg.Enabled, true
	node := bizclient.PunchNode{
		NodeID: s.cfg.NodeID, Name: s.cfg.Name, Host: s.cfg.PublicHost,
		UDPPort: s.cfg.PublicUDPPort, Priority: s.cfg.Priority,
		Enabled: &enabled, Healthy: &healthy,
	}
	backoff := 2 * time.Second
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := client.UpsertPunchNode(ctx, node)
		cancel()
		if err == nil {
			log.Printf("wire punch node registered node=%s host=%s udpPort=%d", s.cfg.NodeID, s.cfg.PublicHost, s.cfg.PublicUDPPort)
			break
		}
		log.Printf("wire punch node registration failed node=%s err=%v retryIn=%s", s.cfg.NodeID, err, backoff)
		time.Sleep(backoff)
		backoff = nextRegistrationBackoff(backoff)
	}
	ticker := time.NewTicker(s.cfg.HeartbeatInterval)
	defer ticker.Stop()
	wasFailing := false
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := client.HeartbeatPunchNode(ctx, s.cfg.NodeID, true)
		cancel()
		if err == nil {
			if wasFailing {
				log.Printf("wire punch node heartbeat recovered node=%s", s.cfg.NodeID)
			}
			wasFailing, backoff = false, 2*time.Second
			continue
		}
		if !wasFailing {
			log.Printf("wire punch node heartbeat failed node=%s err=%v", s.cfg.NodeID, err)
		}
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		registerErr := client.UpsertPunchNode(ctx, node)
		cancel()
		if registerErr == nil {
			log.Printf("wire punch node re-registered after heartbeat failure node=%s", s.cfg.NodeID)
			wasFailing, backoff = false, 2*time.Second
			continue
		}
		wasFailing = true
		log.Printf("wire punch node re-registration failed node=%s err=%v", s.cfg.NodeID, registerErr)
		time.Sleep(backoff)
		backoff = nextRegistrationBackoff(backoff)
	}
}

func nextRegistrationBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > time.Minute {
		return time.Minute
	}
	return next
}

// Handler 返回 punch 服务 HTTP 路由，包含健康检查、端点和协商会话接口。
func (s *Server) Handler() http.Handler {
	s.ensureService()
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "service": "server-wire-punch"})
	})
	mux.HandleFunc("/stats", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, s.service.Stats())
	})
	mux.HandleFunc("/endpoints", s.handleEndpointCollection)
	mux.HandleFunc("/endpoints/", s.handleEndpointItem)
	mux.HandleFunc("/connect-sessions", s.handleConnectSessions)
	mux.HandleFunc("/connect-sessions/", s.handleConnectSessionItem)
	return mux
}

func (s *Server) handlePacket(addr *net.UDPAddr, payload []byte) error {
	s.ensureService()
	var msg UDPMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode punch packet: %w", err)
	}
	switch strings.TrimSpace(msg.Kind) {
	case "ping":
		return s.write(addr, map[string]any{"kind": "pong"})
	case "endpoint_probe", "endpoint_report":
		endpoint, err := s.service.UpsertEndpoint(EndpointReport{
			NetworkID:    msg.NetworkID,
			NodeID:       msg.NodeID,
			Type:         msg.Type,
			Address:      msg.Address,
			NATType:      msg.NATType,
			ObservedFrom: addr.String(),
		})
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
		endpoint, err := s.service.UpsertEndpoint(EndpointReport{
			NetworkID:    req.NetworkID,
			NodeID:       req.NodeID,
			Type:         req.Type,
			Address:      req.Address,
			NATType:      req.NATType,
			ObservedFrom: remoteAddress(r),
			UserAgent:    r.UserAgent(),
		})
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
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/endpoints/"), "/")
	if len(parts) != 2 {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not_found"})
		return
	}
	endpoint, ok := s.service.Endpoint(parts[0], parts[1])
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
	session, err := s.service.CreateConnectSession(ConnectSessionRequest{
		NetworkID:       req.NetworkID,
		RequesterNodeID: req.RequesterNodeID,
		PeerNodeID:      req.PeerNodeID,
		TTLSeconds:      req.TTLSeconds,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	s.notifyConnectSession(session)
	writeJSON(w, http.StatusOK, session)
}

func (s *Server) handleConnectSessionItem(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	sessionID := strings.TrimPrefix(r.URL.Path, "/connect-sessions/")
	session, ok := s.service.Session(sessionID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "session_not_found"})
		return
	}
	writeJSON(w, http.StatusOK, session)
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
	if !s.authorized(r) {
		return false
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
