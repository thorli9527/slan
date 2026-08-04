package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/slan/server/server-wire-relay/internal/admin"
	"github.com/slan/server/server-wire-relay/internal/bizclient"
	"github.com/slan/server/server-wire-relay/internal/config"
	"github.com/slan/server/server-wire-relay/internal/protocol"
	"github.com/slan/server/server-wire-relay/internal/state"
)

// UDPServer 承载 relay_udp 数据面，并同时启动本节点的管理 HTTP 服务。
type UDPServer struct {
	conn              *net.UDPConn
	store             state.StoreAPI
	service           *Service
	adminAddr         string
	cfg               config.Config
	transientErrorsMu sync.Mutex
	transientErrors   map[string]time.Time
}

// NewUDPServer 使用默认内存状态创建 UDP 中继服务。
func NewUDPServer(cfg config.Config) (*UDPServer, error) {
	return NewUDPServerWithStore(cfg, state.NewStore())
}

// NewUDPServerWithStore 使用调用方提供的状态实现创建 UDP 中继服务。
func NewUDPServerWithStore(cfg config.Config, store state.StoreAPI) (*UDPServer, error) {
	addr, err := net.ResolveUDPAddr("udp", cfg.ListenAddr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	_ = conn.SetReadBuffer(cfg.ReadBufferBytes)
	_ = conn.SetWriteBuffer(cfg.WriteBufferBytes)
	if store == nil {
		store = state.NewStore()
	}
	return &UDPServer{
		conn:            conn,
		store:           store,
		service:         NewService(store),
		adminAddr:       cfg.AdminListenAddr,
		cfg:             cfg,
		transientErrors: make(map[string]time.Time),
	}, nil
}

func (s *UDPServer) ensureService() {
	if s.service == nil {
		s.service = NewService(s.store)
	}
}

// Serve 启动管理 HTTP、注册/心跳协程，并在当前 goroutine 中处理 UDP 数据包。
func (s *UDPServer) Serve() error {
	defer s.conn.Close()
	go func() {
		adminServer := &http.Server{
			Addr:    s.adminAddr,
			Handler: admin.New(s.store).Handler(),
		}
		_ = adminServer.ListenAndServe()
	}()
	go s.registerAndHeartbeat()
	workers := s.cfg.PacketWorkers
	if workers <= 0 {
		workers = 1
	}
	errCh := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errCh <- s.servePackets()
		}()
	}
	err := <-errCh
	_ = s.conn.Close()
	wg.Wait()
	return err
}

func (s *UDPServer) servePackets() error {
	buf := make([]byte, 64*1024)
	for {
		n, addr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			return err
		}
		log.Printf(
			"wire relay datagram remote=%s bytes=%d",
			addr.String(),
			n,
		)
		if err := s.handlePacket(addr, buf[:n]); err != nil {
			if !s.shouldReportTransientError(addr, err, time.Now()) {
				continue
			}
			log.Printf("wire relay request failed remote=%s err=%v", addr.String(), err)
			_ = s.write(addr, protocol.ServerMessage{
				Kind: "error",
				Error: &protocol.ErrorResponse{
					Code:    "request_failed",
					Message: err.Error(),
				},
			})
		}
	}
}

const transientErrorReportInterval = 10 * time.Second

func (s *UDPServer) shouldReportTransientError(addr *net.UDPAddr, err error, now time.Time) bool {
	if !errors.Is(err, state.ErrPeerNotAttached) && !errors.Is(err, state.ErrParticipantNotFound) {
		return true
	}
	key := addr.String()
	if errors.Is(err, state.ErrPeerNotAttached) {
		key += "|peer"
	} else {
		key += "|participant"
	}
	s.transientErrorsMu.Lock()
	defer s.transientErrorsMu.Unlock()
	if s.transientErrors == nil {
		s.transientErrors = make(map[string]time.Time)
	}
	if previous, exists := s.transientErrors[key]; exists && now.Sub(previous) < transientErrorReportInterval {
		return false
	}
	s.transientErrors[key] = now
	if len(s.transientErrors) > 4096 {
		cutoff := now.Add(-time.Minute)
		for item, seenAt := range s.transientErrors {
			if seenAt.Before(cutoff) {
				delete(s.transientErrors, item)
			}
		}
	}
	return true
}

func (s *UDPServer) registerAndHeartbeat() {
	client := bizclient.New(s.cfg.BizURL, s.cfg.InternalWireToken)
	if !client.Enabled() {
		log.Printf("wire relay biz registration disabled: missing SLAN_BIZ_URL or SLAN_INTERNAL_WIRE_TOKEN")
		return
	}
	enabled := s.cfg.Enabled
	healthy := true
	node := bizclient.RelayNode{
		RegionID:          s.cfg.RegionID,
		NodeID:            s.cfg.NodeID,
		Host:              s.cfg.PublicHost,
		UDPPort:           s.cfg.PublicUDPPort,
		AdminPort:         s.cfg.PublicAdminPort,
		Enabled:           &enabled,
		Healthy:           &healthy,
		Priority:          s.cfg.Priority,
		TicketKeyRotation: relayTicketKeyStatus(),
	}
	backoff := 2 * time.Second
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := client.UpsertRelayNode(ctx, node)
		cancel()
		if err == nil {
			log.Printf("wire relay node registered region=%s node=%s host=%s udpPort=%d", s.cfg.RegionID, s.cfg.NodeID, s.cfg.PublicHost, s.cfg.PublicUDPPort)
			backoff = 2 * time.Second
			break
		}
		log.Printf("wire relay node registration failed region=%s node=%s err=%v retryIn=%s", s.cfg.RegionID, s.cfg.NodeID, err, backoff)
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
		err := client.HeartbeatRelayNode(ctx, s.cfg.RegionID, s.cfg.NodeID, true, relayTicketKeyStatus())
		cancel()
		if err != nil {
			if !wasFailing {
				log.Printf("wire relay node heartbeat failed region=%s node=%s err=%v", s.cfg.RegionID, s.cfg.NodeID, err)
			}
			node.TicketKeyRotation = relayTicketKeyStatus()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			registerErr := client.UpsertRelayNode(ctx, node)
			cancel()
			if registerErr == nil {
				log.Printf("wire relay node re-registered after heartbeat failure region=%s node=%s", s.cfg.RegionID, s.cfg.NodeID)
				wasFailing = false
				backoff = 2 * time.Second
				continue
			}
			log.Printf("wire relay node re-registration failed region=%s node=%s err=%v", s.cfg.RegionID, s.cfg.NodeID, registerErr)
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
			log.Printf("wire relay node heartbeat recovered region=%s node=%s", s.cfg.RegionID, s.cfg.NodeID)
			wasFailing = false
		}
		backoff = 2 * time.Second
	}
}

func relayTicketKeyStatus() bizclient.TicketKeyStatus {
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

func (s *UDPServer) handlePacket(addr *net.UDPAddr, payload []byte) error {
	return s.handlePacketWithWriter(addr, payload, s.write)
}

func (s *UDPServer) handlePacketWithWriter(addr *net.UDPAddr, payload []byte, writer func(*net.UDPAddr, protocol.ServerMessage) error) error {
	s.ensureService()
	var msg protocol.ClientMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode client message: %w", err)
	}
	switch msg.Kind {
	case "ping":
		if msg.SessionID != "" && msg.ParticipantID != "" {
			if err := s.service.RefreshParticipant(addr, msg.SessionID, msg.ParticipantID); err != nil {
				return fmt.Errorf(
					"refresh remote=%s session=%s participant=%s: %w",
					addr.String(),
					msg.SessionID,
					msg.ParticipantID,
					err,
				)
			}
		}
		return writer(addr, protocol.ServerMessage{Kind: "pong"})
	case "attach":
		if msg.Ticket == nil {
			return errors.New("attach ticket is required")
		}
		peerID, err := s.service.Attach(addr, msg.ParticipantID, *msg.Ticket, msg.Transport)
		if err != nil {
			return fmt.Errorf(
				"attach remote=%s participant=%s session=%s transport=%s: %w",
				addr.String(),
				msg.ParticipantID,
				msg.Ticket.SessionID,
				msg.Transport,
				err,
			)
		}
		return writer(addr, protocol.ServerMessage{
			Kind:              "attached",
			SessionID:         msg.Ticket.SessionID,
			ParticipantID:     msg.ParticipantID,
			PeerParticipantID: peerID,
		})
	case "forward":
		peerAddr, peerID, err := s.service.Forward(addr, msg.SessionID, msg.ParticipantID, msg.Payload)
		if err != nil {
			return fmt.Errorf(
				"forward remote=%s session=%s participant=%s payloadBytes=%d: %w",
				addr.String(),
				msg.SessionID,
				msg.ParticipantID,
				len(msg.Payload),
				err,
			)
		}
		if err := writer(peerAddr, protocol.ServerMessage{
			Kind:          "packet",
			SessionID:     msg.SessionID,
			ParticipantID: msg.ParticipantID,
			Payload:       msg.Payload,
		}); err != nil {
			return err
		}
		if !s.cfg.ForwardAckEnabled {
			return nil
		}
		return writer(addr, protocol.ServerMessage{
			Kind:           "forwarded",
			SessionID:      msg.SessionID,
			ParticipantID:  peerID,
			BytesForwarded: len(msg.Payload),
		})
	case "detach":
		if err := s.service.Detach(addr, msg.SessionID, msg.ParticipantID); err != nil {
			return err
		}
		return writer(addr, protocol.ServerMessage{
			Kind:          "detached",
			SessionID:     msg.SessionID,
			ParticipantID: msg.ParticipantID,
		})
	default:
		return fmt.Errorf("unsupported kind %q", msg.Kind)
	}
}

func (s *UDPServer) write(addr *net.UDPAddr, body protocol.ServerMessage) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	log.Printf(
		"wire relay write remote=%s kind=%s session=%s participant=%s payloadBytes=%d bytes=%d",
		addr.String(),
		body.Kind,
		body.SessionID,
		body.ParticipantID,
		len(body.Payload),
		len(payload),
	)
	_, err = s.conn.WriteToUDP(payload, addr)
	return err
}
