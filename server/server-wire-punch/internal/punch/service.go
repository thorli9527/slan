package punch

import (
	"errors"
	"strings"
	"time"

	"github.com/slan/server/server-wire-punch/internal/config"
)

// Service 承载 punch 节点的端点登记、查询和连接协商业务实现。
// Server 负责 HTTP/UDP 协议入口和 UDP 通知发送，Service 负责参数校验和仓储编排。
type Service struct {
	store Repository
	cfg   config.Config
}

type EndpointReport struct {
	NetworkID    string
	NodeID       string
	Type         string
	Address      string
	NATType      string
	ObservedFrom string
	UserAgent    string
}

type ConnectSessionRequest struct {
	NetworkID       string
	RequesterNodeID string
	PeerNodeID      string
	TTLSeconds      int
}

func NewService(store Repository, cfg config.Config) *Service {
	return &Service{store: store, cfg: cfg}
}

func (s *Service) Stats() Stats {
	return s.store.Stats()
}

func (s *Service) UpsertEndpoint(req EndpointReport) (Endpoint, error) {
	networkID := strings.TrimSpace(req.NetworkID)
	nodeID := strings.TrimSpace(req.NodeID)
	if networkID == "" || nodeID == "" {
		return Endpoint{}, errors.New("networkId and nodeId are required")
	}
	now := time.Now()
	reflexive := strings.TrimSpace(req.ObservedFrom)
	if reflexive == "" {
		reflexive = strings.TrimSpace(req.Address)
	}
	endpoint := Endpoint{
		NetworkID:    networkID,
		NodeID:       nodeID,
		Type:         defaultString(req.Type, "reflexive"),
		Address:      strings.TrimSpace(req.Address),
		Reflexive:    reflexive,
		NATType:      defaultString(req.NATType, "unknown"),
		UpdatedAt:    now,
		ExpiresAt:    now.Add(s.cfg.EndpointTTL),
		UserAgent:    req.UserAgent,
		ObservedFrom: strings.TrimSpace(req.ObservedFrom),
	}
	if endpoint.Address == "" {
		endpoint.Address = endpoint.Reflexive
	}
	return s.store.PutEndpoint(endpoint), nil
}

func (s *Service) Endpoint(networkID, nodeID string) (Endpoint, bool) {
	return s.store.Endpoint(networkID, nodeID)
}

func (s *Service) CreateConnectSession(req ConnectSessionRequest) (ConnectSession, error) {
	if strings.TrimSpace(req.NetworkID) == "" || strings.TrimSpace(req.RequesterNodeID) == "" || strings.TrimSpace(req.PeerNodeID) == "" {
		return ConnectSession{}, errors.New("networkId requesterNodeId peerNodeId are required")
	}
	ttl := s.cfg.SessionTTL
	if req.TTLSeconds > 0 {
		ttl = time.Duration(req.TTLSeconds) * time.Second
	}
	return s.store.CreateSession(req.NetworkID, req.RequesterNodeID, req.PeerNodeID, ttl), nil
}

func (s *Service) Session(sessionID string) (ConnectSession, bool) {
	return s.store.Session(sessionID)
}
