package punch

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

// Endpoint 表示某个虚拟网络内，一个节点最近一次被打洞服务观测到的地址信息。
type Endpoint struct {
	// NetworkID 是虚拟网络 ID，用于隔离不同网络内的节点端点。
	NetworkID string `json:"networkId"`
	// NodeID 是客户端节点或设备 ID。
	NodeID string `json:"nodeId"`
	// Type 表示端点来源或类别，例如 reflexive/local。
	Type string `json:"type"`
	// Address 是客户端主动上报的候选地址。
	Address string `json:"address"`
	// Reflexive 是服务端从 UDP 包源地址观测到的公网反射地址。
	Reflexive string `json:"reflexive"`
	// NATType 是客户端侧识别到的 NAT 类型，未知时通常为 unknown。
	NATType string `json:"natType"`
	// UpdatedAt 是端点被服务端接受或刷新的时间。
	UpdatedAt time.Time `json:"updatedAt"`
	// ExpiresAt 是端点失效时间，过期后不会参与直连协商。
	ExpiresAt time.Time `json:"expiresAt"`
	// UserAgent 记录 HTTP 上报方的客户端信息，UDP 上报通常为空。
	UserAgent string `json:"userAgent,omitempty"`
	// ObservedFrom 是打洞服务看到的原始远端地址。
	ObservedFrom string `json:"observedFrom,omitempty"`
}

// ConnectSession 表示一次短生命周期的 P2P 直连协商会话。
type ConnectSession struct {
	// SessionID 是协商会话的唯一标识。
	SessionID string `json:"sessionId"`
	// NetworkID 是双方节点所属的虚拟网络。
	NetworkID string `json:"networkId"`
	// RequesterNodeID 是发起直连请求的节点。
	RequesterNodeID string `json:"requesterNodeId"`
	// PeerNodeID 是被请求直连的目标节点。
	PeerNodeID string `json:"peerNodeId"`
	// Requester 是发起方当前可用端点快照，未知时为空。
	Requester *Endpoint `json:"requester,omitempty"`
	// Peer 是目标方当前可用端点快照，未知时为空。
	Peer *Endpoint `json:"peer,omitempty"`
	// ExpiresAt 是客户端应停止使用该协商结果的时间。
	ExpiresAt time.Time `json:"expiresAt"`
	// CreatedAt 是会话创建时间。
	CreatedAt time.Time `json:"createdAt"`
}

// Store 是打洞服务默认内存仓储，保存短生命周期的端点和协商会话。
type Store struct {
	mu        sync.Mutex
	endpoints map[string]Endpoint
	sessions  map[string]ConnectSession
}

// Stats 是打洞节点暴露给管理接口的运行统计。
type Stats struct {
	// EndpointCount 是当前未过期端点数量。
	EndpointCount int `json:"endpointCount"`
	// SessionCount 是当前未过期协商会话数量。
	SessionCount int `json:"sessionCount"`
	// UpdatedAt 是统计生成时间。
	UpdatedAt time.Time `json:"updatedAt"`
}

func NewStore() *Store {
	return &Store{
		endpoints: make(map[string]Endpoint),
		sessions:  make(map[string]ConnectSession),
	}
}

func (s *Store) PutEndpoint(endpoint Endpoint) Endpoint {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(time.Now())
	s.endpoints[endpointKey(endpoint.NetworkID, endpoint.NodeID)] = endpoint
	return endpoint
}

func (s *Store) Endpoint(networkID, nodeID string) (Endpoint, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.cleanupLocked(now)
	endpoint, ok := s.endpoints[endpointKey(networkID, nodeID)]
	if !ok || endpoint.ExpiresAt.Before(now) {
		return Endpoint{}, false
	}
	return endpoint, true
}

func (s *Store) CreateSession(networkID, requesterNodeID, peerNodeID string, ttl time.Duration) ConnectSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.cleanupLocked(now)
	requester, requesterOK := s.endpoints[endpointKey(networkID, requesterNodeID)]
	peer, peerOK := s.endpoints[endpointKey(networkID, peerNodeID)]
	session := ConnectSession{
		SessionID:       randomID(),
		NetworkID:       networkID,
		RequesterNodeID: requesterNodeID,
		PeerNodeID:      peerNodeID,
		CreatedAt:       now,
		ExpiresAt:       now.Add(ttl),
	}
	if requesterOK && requester.ExpiresAt.After(now) {
		value := requester
		session.Requester = &value
	}
	if peerOK && peer.ExpiresAt.After(now) {
		value := peer
		session.Peer = &value
	}
	s.sessions[session.SessionID] = session
	return session
}

func (s *Store) Session(sessionID string) (ConnectSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.cleanupLocked(now)
	session, ok := s.sessions[strings.TrimSpace(sessionID)]
	if !ok || session.ExpiresAt.Before(now) {
		return ConnectSession{}, false
	}
	return session, true
}

func (s *Store) Stats() Stats {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.cleanupLocked(now)
	return Stats{
		EndpointCount: len(s.endpoints),
		SessionCount:  len(s.sessions),
		UpdatedAt:     now,
	}
}

func (s *Store) cleanupLocked(now time.Time) {
	for key, endpoint := range s.endpoints {
		if endpoint.ExpiresAt.Before(now) {
			delete(s.endpoints, key)
		}
	}
	for key, session := range s.sessions {
		if session.ExpiresAt.Before(now) {
			delete(s.sessions, key)
		}
	}
}

func endpointKey(networkID, nodeID string) string {
	return strings.TrimSpace(networkID) + ":" + strings.TrimSpace(nodeID)
}

func randomID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(bytes[:])
}
