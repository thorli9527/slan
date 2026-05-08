package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/slan/server/server-biz/configs"
	"github.com/slan/server/server-biz/internal/mqttauth"
	"github.com/slan/server/server-biz/internal/repo"
)

const (
	probeMagic         = "SLAN_ICE_PROBE"
	probeResponseMagic = "SLAN_ICE_PROBE_RESP"
)

type probeRequest struct {
	Magic       string `json:"magic"`
	Version     int    `json:"version"`
	ClientID    string `json:"clientId"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	PeerID      string `json:"peerId,omitempty"`
	Nonce       int64  `json:"nonce"`
	TimestampMS int64  `json:"timestampMs"`
}

type probeResponse struct {
	Magic        string `json:"magic"`
	Version      int    `json:"version"`
	ServerID     string `json:"serverId,omitempty"`
	ObservedAddr string `json:"observedAddr,omitempty"`
	PeerID       string `json:"peerId,omitempty"`
	Nonce        int64  `json:"nonce"`
	TimestampMS  int64  `json:"timestampMs"`
	Error        string `json:"error,omitempty"`
}

func main() {
	configPath := flag.String("config", "", "path to server-biz config yaml")
	flag.Parse()

	cfg, err := configs.LoadConfig(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	if !cfg.MQTT.Enabled {
		log.Fatal("ice-server requires mqtt.enabled=true so MQTT credentials can be validated")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	runtime, err := configs.InitPostgresRuntime(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer runtime.Close()

	s := &server{
		cfg:  cfg,
		repo: repo.NewPostgresRepository(runtime.Postgres),
	}
	if err := s.run(ctx); err != nil {
		log.Fatal(err)
	}
}

type server struct {
	cfg  configs.Config
	repo *repo.PostgresRepository
}

func (s *server) run(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", s.cfg.IceProbe.BindAddress)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	maxPacketBytes := s.cfg.IceProbe.MaxPacketBytes
	if maxPacketBytes <= 0 {
		maxPacketBytes = 2048
	}
	log.Printf("slan ice-server listening on udp %s server_id=%s", conn.LocalAddr(), s.cfg.IceProbe.ServerID)
	buf := make([]byte, maxPacketBytes)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("ice read error: %v", err)
			continue
		}
		payload := append([]byte(nil), buf[:n]...)
		go s.handlePacket(ctx, conn, remote, payload)
	}
}

func (s *server) handlePacket(ctx context.Context, conn *net.UDPConn, remote *net.UDPAddr, payload []byte) {
	var req probeRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		s.writeResponse(conn, remote, probeResponse{Error: "invalid_json"})
		return
	}
	resp := probeResponse{
		Magic:       probeResponseMagic,
		Version:     1,
		ServerID:    strings.TrimSpace(s.cfg.IceProbe.ServerID),
		PeerID:      strings.TrimSpace(req.PeerID),
		Nonce:       req.Nonce,
		TimestampMS: time.Now().UnixMilli(),
	}
	if req.Magic != probeMagic || req.Version != 1 {
		resp.Error = "invalid_probe"
		s.writeResponse(conn, remote, resp)
		return
	}
	deviceID, ok := mqttauth.ValidateDevice(s.cfg.MQTT, strings.TrimSpace(req.ClientID), strings.TrimSpace(req.Username), strings.TrimSpace(req.Password))
	if !ok {
		resp.Error = "unauthorized"
		s.writeResponse(conn, remote, resp)
		return
	}
	device, err := s.repo.GetDeviceByID(ctx, deviceID)
	if err != nil || device.DeviceID == "" {
		resp.Error = "device_not_found"
		s.writeResponse(conn, remote, resp)
		return
	}
	if peerID := strings.TrimSpace(req.PeerID); peerID != "" {
		node, err := s.repo.GetNodeByID(ctx, peerID)
		if err != nil || node.NodeID == "" {
			resp.Error = "peer_not_found"
			s.writeResponse(conn, remote, resp)
			return
		}
		if node.DeviceID != device.DeviceID {
			resp.Error = "peer_device_mismatch"
			s.writeResponse(conn, remote, resp)
			return
		}
	}
	resp.ObservedAddr = remote.String()
	s.writeResponse(conn, remote, resp)
}

func (s *server) writeResponse(conn *net.UDPConn, remote *net.UDPAddr, resp probeResponse) {
	if resp.Magic == "" {
		resp.Magic = probeResponseMagic
	}
	if resp.Version == 0 {
		resp.Version = 1
	}
	if resp.TimestampMS == 0 {
		resp.TimestampMS = time.Now().UnixMilli()
	}
	payload, err := json.Marshal(resp)
	if err != nil {
		return
	}
	if _, err := conn.WriteToUDP(payload, remote); err != nil {
		log.Printf("ice write error to %s: %v", remote, err)
	}
}
