package derp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/slan/server/server-wire-derp/internal/admin"
	"github.com/slan/server/server-wire-derp/internal/bizclient"
	"github.com/slan/server/server-wire-derp/internal/config"
	"github.com/slan/server/server-wire-derp/internal/protocol"
	"github.com/slan/server/server-wire-derp/internal/state"
)

// Server 承载 DERP TCP 数据面，并同时启动本节点的管理 HTTP 服务。
type Server struct {
	listener  net.Listener
	store     state.StoreAPI
	service   *Service
	adminAddr string
	cfg       config.Config

	mu      sync.RWMutex
	writers map[string]*peerWriter
}

type peerWriter struct {
	mu      sync.Mutex
	encoder *json.Encoder
}

func newPeerWriter(conn net.Conn) *peerWriter {
	return &peerWriter{encoder: json.NewEncoder(conn)}
}

func (w *peerWriter) Encode(msg protocol.ServerMessage) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.encoder.Encode(msg)
}

// NewServer 使用默认内存状态创建 DERP 服务。
func NewServer(cfg config.Config) (*Server, error) {
	return NewServerWithStore(cfg, state.NewStore())
}

// NewServerWithStore 使用调用方提供的状态实现创建 DERP 服务。
func NewServerWithStore(cfg config.Config, store state.StoreAPI) (*Server, error) {
	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		return nil, err
	}
	if store == nil {
		store = state.NewStore()
	}
	return &Server{
		listener:  listener,
		store:     store,
		service:   NewService(store),
		adminAddr: cfg.AdminListenAddr,
		cfg:       cfg,
		writers:   make(map[string]*peerWriter),
	}, nil
}

func (s *Server) ensureService() {
	if s.service == nil {
		s.service = NewService(s.store)
	}
}

// Serve 启动管理 HTTP、注册/心跳协程，并循环接受客户端 TCP 连接。
func (s *Server) Serve() error {
	defer s.listener.Close()
	go func() {
		httpServer := &http.Server{
			Addr:              s.adminAddr,
			Handler:           admin.New(s.store).Handler(),
			ReadHeaderTimeout: 5 * time.Second,
		}
		_ = httpServer.ListenAndServe()
	}()
	go s.registerAndHeartbeat()
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return err
		}
		go s.handleConn(conn)
	}
}

func (s *Server) registerAndHeartbeat() {
	client := bizclient.New(s.cfg.BizURL, s.cfg.InternalWireToken)
	if !client.Enabled() {
		log.Printf("wire derp biz registration disabled: missing SLAN_BIZ_URL or SLAN_INTERNAL_WIRE_TOKEN")
		return
	}
	enabled := s.cfg.Enabled
	healthy := true
	node := bizclient.DerpNode{
		RegionID:          s.cfg.RegionID,
		NodeID:            s.cfg.NodeID,
		Name:              s.cfg.RegionID,
		Host:              s.cfg.PublicHost,
		Port:              s.cfg.PublicPort,
		Enabled:           &enabled,
		Healthy:           &healthy,
		Priority:          s.cfg.Priority,
		TicketKeyRotation: derpTicketKeyStatus(),
	}
	backoff := 2 * time.Second
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := client.UpsertDerpNode(ctx, node)
		cancel()
		if err == nil {
			log.Printf("wire derp node registered region=%s node=%s host=%s port=%d", s.cfg.RegionID, s.cfg.NodeID, s.cfg.PublicHost, s.cfg.PublicPort)
			backoff = 2 * time.Second
			break
		}
		log.Printf("wire derp node registration failed region=%s node=%s err=%v retryIn=%s", s.cfg.RegionID, s.cfg.NodeID, err, backoff)
		time.Sleep(backoff)
		if backoff < time.Minute {
			backoff *= 2
			if backoff > time.Minute {
				backoff = time.Minute
			}
		}
	}
	ticker := time.NewTicker(s.cfg.HeartbeatInterval)
	defer ticker.Stop()
	wasFailing := false
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := client.HeartbeatDerpNode(ctx, s.cfg.RegionID, s.cfg.NodeID, true, derpTicketKeyStatus())
		cancel()
		if err != nil {
			if !wasFailing {
				log.Printf("wire derp node heartbeat failed region=%s node=%s err=%v", s.cfg.RegionID, s.cfg.NodeID, err)
			}
			node.TicketKeyRotation = derpTicketKeyStatus()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			registerErr := client.UpsertDerpNode(ctx, node)
			cancel()
			if registerErr == nil {
				log.Printf("wire derp node re-registered after heartbeat failure region=%s node=%s", s.cfg.RegionID, s.cfg.NodeID)
				wasFailing = false
				backoff = 2 * time.Second
				continue
			}
			log.Printf("wire derp node re-registration failed region=%s node=%s err=%v", s.cfg.RegionID, s.cfg.NodeID, registerErr)
			wasFailing = true
			time.Sleep(backoff)
			if backoff < time.Minute {
				backoff *= 2
				if backoff > time.Minute {
					backoff = time.Minute
				}
			}
			continue
		}
		if wasFailing {
			log.Printf("wire derp node heartbeat recovered region=%s node=%s", s.cfg.RegionID, s.cfg.NodeID)
			wasFailing = false
		}
		backoff = 2 * time.Second
	}
}

func derpTicketKeyStatus() bizclient.TicketKeyStatus {
	current := state.CurrentTicketKeyStatus()
	return bizclient.TicketKeyStatus{
		Source:             current.Source,
		KeyRingID:          current.KeyRingID,
		SigningConfigured:  current.SigningConfigured,
		KeyRingConfigured:  current.KeyRingConfigured,
		EffectiveKeyCount:  current.EffectiveKeyCount,
		RotationReady:      current.RotationReady,
		AcceptsDevFallback: current.AcceptsDevFallback,
	}
}

func (s *Server) handleConn(conn net.Conn) {
	s.ensureService()
	defer conn.Close()
	configureTCPConn(conn, s.cfg)
	reader := bufio.NewScanner(conn)
	reader.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	writer := newPeerWriter(conn)
	var currentPeerID string
	for reader.Scan() {
		var msg protocol.ClientMessage
		if err := json.Unmarshal(reader.Bytes(), &msg); err != nil {
			_ = writer.Encode(protocol.ServerMessage{Kind: "error", Error: &protocol.ErrorResponse{Code: "invalid_json", Message: err.Error()}})
			return
		}
		switch msg.Kind {
		case "connect":
			if msg.Ticket == nil {
				_ = writer.Encode(protocol.ServerMessage{Kind: "error", Error: &protocol.ErrorResponse{Code: "ticket_required", Message: "connect ticket is required"}})
				return
			}
			session, renewAfter, err := s.service.Connect(conn, msg.PeerID, msg.NodeID, msg.RegionID, *msg.Ticket)
			if err != nil {
				_ = writer.Encode(protocol.ServerMessage{Kind: "error", Error: &protocol.ErrorResponse{Code: "connect_failed", Message: err.Error()}})
				return
			}
			currentPeerID = msg.PeerID
			replaced := s.setWriter(currentPeerID, writer)
			log.Printf("derp peer connected peer=%s session=%s node=%s region=%s replaced=%t", currentPeerID, session.SessionID, session.NodeID, session.RegionID, replaced)
			_ = writer.Encode(protocol.ServerMessage{
				Kind:         "connected",
				SessionID:    session.SessionID,
				PeerID:       currentPeerID,
				RegionID:     session.RegionID,
				NodeID:       session.NodeID,
				RenewAfterMs: renewAfter,
			})
		case "send":
			if currentPeerID == "" {
				_ = writer.Encode(protocol.ServerMessage{Kind: "error", Error: &protocol.ErrorResponse{Code: "connect_required", Message: "connect is required before send"}})
				continue
			}
			if !s.isCurrentWriter(currentPeerID, writer) {
				_ = writer.Encode(protocol.ServerMessage{Kind: "error", Error: &protocol.ErrorResponse{Code: "connection_superseded", Message: "peer connection has been superseded by a newer connection"}})
				return
			}
			if msg.TargetPeerID == "" {
				_ = writer.Encode(protocol.ServerMessage{Kind: "error", Error: &protocol.ErrorResponse{Code: "target_required", Message: "targetPeerId is required"}})
				continue
			}
			s.service.TouchPeer(currentPeerID)
			session, err := s.service.BindSessionPeer(msg.SessionID, currentPeerID, msg.TargetPeerID)
			if err != nil {
				_ = writer.Encode(protocol.ServerMessage{Kind: "error", Error: &protocol.ErrorResponse{Code: "session_not_found", Message: err.Error()}})
				continue
			}
			target, ok := s.writer(msg.TargetPeerID)
			if !ok {
				log.Printf("derp forward target missing source=%s target=%s session=%s", currentPeerID, msg.TargetPeerID, session.SessionID)
				_ = writer.Encode(protocol.ServerMessage{Kind: "error", Error: &protocol.ErrorResponse{Code: "target_not_connected", Message: fmt.Sprintf("target peer %s is not connected to this DERP node", msg.TargetPeerID)}})
				continue
			}
			if err := target.Encode(protocol.ServerMessage{
				Kind:         "recv",
				SessionID:    session.SessionID,
				SourcePeerID: currentPeerID,
				Payload:      msg.Payload,
			}); err != nil {
				log.Printf("derp forward failed source=%s target=%s session=%s bytes=%d err=%v", currentPeerID, msg.TargetPeerID, session.SessionID, len(msg.Payload), err)
				if s.clearWriter(msg.TargetPeerID, target) {
					s.service.Disconnect(msg.TargetPeerID)
				}
				_ = writer.Encode(protocol.ServerMessage{Kind: "error", Error: &protocol.ErrorResponse{Code: "target_not_connected", Message: fmt.Sprintf("target peer %s connection is unavailable", msg.TargetPeerID)}})
				continue
			}
			if s.cfg.SendAckEnabled {
				_ = writer.Encode(protocol.ServerMessage{
					Kind:           "sent",
					SessionID:      session.SessionID,
					BytesForwarded: len(msg.Payload),
				})
			}
		case "disconnect":
			if msg.SessionID != "" {
				s.service.TouchPeer(currentPeerID)
			}
			_ = writer.Encode(protocol.ServerMessage{Kind: "disconnected", SessionID: msg.SessionID})
			if s.clearWriter(currentPeerID, writer) {
				s.service.Disconnect(currentPeerID)
			}
			return
		default:
			_ = writer.Encode(protocol.ServerMessage{Kind: "error", Error: &protocol.ErrorResponse{Code: "unsupported_kind", Message: fmt.Sprintf("unsupported kind %q", msg.Kind)}})
		}
	}
	if s.clearWriter(currentPeerID, writer) {
		s.service.Disconnect(currentPeerID)
	}
}

func configureTCPConn(conn net.Conn, cfg config.Config) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tcpConn.SetNoDelay(true)
	_ = tcpConn.SetKeepAlive(true)
	_ = tcpConn.SetKeepAlivePeriod(30 * time.Second)
	_ = tcpConn.SetReadBuffer(cfg.ReadBufferBytes)
	_ = tcpConn.SetWriteBuffer(cfg.WriteBufferBytes)
}

func (s *Server) setWriter(peerID string, writer *peerWriter) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, replaced := s.writers[peerID]
	s.writers[peerID] = writer
	return replaced
}

func (s *Server) writer(peerID string) (*peerWriter, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	writer, ok := s.writers[peerID]
	return writer, ok
}

func (s *Server) isCurrentWriter(peerID string, writer *peerWriter) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return peerID != "" && s.writers[peerID] == writer
}

func (s *Server) clearWriter(peerID string, writer *peerWriter) bool {
	if peerID == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if current := s.writers[peerID]; current != writer {
		return false
	}
	delete(s.writers, peerID)
	return true
}
