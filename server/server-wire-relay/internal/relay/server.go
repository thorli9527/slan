package relay

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/slan/server/server-wire-relay/internal/admin"
	"github.com/slan/server/server-wire-relay/internal/bizclient"
	"github.com/slan/server/server-wire-relay/internal/config"
	"github.com/slan/server/server-wire-relay/internal/protocol"
	"github.com/slan/server/server-wire-relay/internal/state"
)

type UDPServer struct {
	conn      *net.UDPConn
	store     *state.Store
	adminAddr string
	cfg       config.Config
}

func NewUDPServer(cfg config.Config) (*UDPServer, error) {
	addr, err := net.ResolveUDPAddr("udp", cfg.ListenAddr)
	if err != nil {
		return nil, err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return nil, err
	}
	return &UDPServer{
		conn:      conn,
		store:     state.NewStore(),
		adminAddr: cfg.AdminListenAddr,
		cfg:       cfg,
	}, nil
}

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
	buf := make([]byte, 64*1024)
	for {
		n, addr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			return err
		}
		if err := s.handlePacket(addr, buf[:n]); err != nil {
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
	var msg protocol.ClientMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return fmt.Errorf("decode client message: %w", err)
	}
	switch msg.Kind {
	case "ping":
		if msg.SessionID != "" && msg.ParticipantID != "" {
			if err := s.store.RefreshParticipant(addr, msg.SessionID, msg.ParticipantID); err != nil {
				return err
			}
		}
		return writer(addr, protocol.ServerMessage{Kind: "pong"})
	case "attach":
		if msg.Ticket == nil {
			return errors.New("attach ticket is required")
		}
		_, peerID, err := s.store.Attach(addr, msg.ParticipantID, *msg.Ticket, msg.Transport)
		if err != nil {
			return err
		}
		return writer(addr, protocol.ServerMessage{
			Kind:              "attached",
			SessionID:         msg.Ticket.SessionID,
			ParticipantID:     msg.ParticipantID,
			PeerParticipantID: peerID,
		})
	case "forward":
		peerAddr, peerID, err := s.store.Forward(addr, msg.SessionID, msg.ParticipantID, msg.Payload)
		if err != nil {
			return err
		}
		if err := writer(peerAddr, protocol.ServerMessage{
			Kind:          "packet",
			SessionID:     msg.SessionID,
			ParticipantID: msg.ParticipantID,
			Payload:       msg.Payload,
		}); err != nil {
			return err
		}
		return writer(addr, protocol.ServerMessage{
			Kind:           "forwarded",
			SessionID:      msg.SessionID,
			ParticipantID:  peerID,
			BytesForwarded: len(msg.Payload),
		})
	case "detach":
		if err := s.store.Detach(addr, msg.SessionID, msg.ParticipantID); err != nil {
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
	_, err = s.conn.WriteToUDP(payload, addr)
	return err
}
