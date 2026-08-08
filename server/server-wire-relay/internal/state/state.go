package state

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/slan/server/server-wire-relay/internal/protocol"
)

var (
	ErrSessionNotFound       = errors.New("session not found")
	ErrParticipantNotFound   = errors.New("participant not attached")
	ErrPeerNotAttached       = errors.New("peer not attached")
	ErrTransportUnsupported  = errors.New("unsupported transport")
	ErrTicketInvalid         = errors.New("invalid ticket")
	ErrSourceAlreadyAttached = errors.New("source already attached")
)

const participantIdleTimeout = 90 * time.Second

type Session struct {
	// ID 是 relay ticket 下发的中继会话 ID。
	ID string
	// PeerID 是当前参与方预期连接的对端节点 ID。
	PeerID string
	// Path 标记当前会话使用的中继路径，现阶段为 relay_udp。
	Path string
	// Participants 保存参与方 ID 到 UDP 源地址的绑定。
	Participants map[string]*net.UDPAddr
	// ParticipantLastSeen 保存参与方最后一次 attach、keepalive 或发包时间。
	ParticipantLastSeen map[string]time.Time
	// ExpiresAt 是 ticket 控制的会话失效时间。
	ExpiresAt time.Time
}

// SessionView 是管理 HTTP 接口返回的中继会话只读视图。
type SessionView struct {
	// ID 是中继会话 ID。
	ID string `json:"id"`
	// PeerID 是会话对端节点 ID。
	PeerID string `json:"peerId"`
	// Path 是转发路径类型。
	Path string `json:"path"`
	// Participants 是参与方 ID 到远端地址字符串的映射。
	Participants map[string]string `json:"participants"`
	// ParticipantCount 是当前已绑定参与方数量。
	ParticipantCount int `json:"participantCount"`
	// ExpiresAt 是会话过期时间。
	ExpiresAt time.Time `json:"expiresAt,omitempty"`
}

// Metrics 汇总 UDP 中继运行时计数器，供管理接口和巡检使用。
type Metrics struct {
	// SessionCount 是当前会话数量。
	SessionCount int `json:"sessionCount"`
	// SourceBindingCount 是 UDP 源地址绑定数量。
	SourceBindingCount int `json:"sourceBindingCount"`
	// AttachCount 是累计成功 attach 次数。
	AttachCount uint64 `json:"attachCount"`
	// ParticipantRefreshCount 是参与方地址刷新次数。
	ParticipantRefreshCount uint64 `json:"participantRefreshCount"`
	// ParticipantAddressChangeCount 是参与方地址发生变化的次数。
	ParticipantAddressChangeCount uint64 `json:"participantAddressChangeCount"`
	// ForwardCount 是成功找到对端并转发的次数。
	ForwardCount uint64 `json:"forwardCount"`
	// ForwardPeerNotAttachedCount 是转发时对端尚未 attach 的次数。
	ForwardPeerNotAttachedCount uint64 `json:"forwardPeerNotAttachedCount"`
	// LastRefreshSessionID 是最近一次 refresh 命中的 session。
	LastRefreshSessionID string `json:"lastRefreshSessionId,omitempty"`
	// LastRefreshParticipantID 是最近一次 refresh 命中的 participant。
	LastRefreshParticipantID string `json:"lastRefreshParticipantId,omitempty"`
	// LastRefreshAddr 是最近一次 refresh 后记录的源地址。
	LastRefreshAddr string `json:"lastRefreshAddr,omitempty"`
	// LastForwardSessionID 是最近一次 forward 命中的 session。
	LastForwardSessionID string `json:"lastForwardSessionId,omitempty"`
	// LastForwardParticipantID 是最近一次发起 forward 的 participant。
	LastForwardParticipantID string `json:"lastForwardParticipantId,omitempty"`
	// LastForwardSourceAddr 是最近一次 forward 的请求源地址。
	LastForwardSourceAddr string `json:"lastForwardSourceAddr,omitempty"`
	// LastForwardPeerID 是最近一次 forward 选中的对端 participant。
	LastForwardPeerID string `json:"lastForwardPeerId,omitempty"`
	// LastForwardPeerAddr 是最近一次 forward 返回的对端地址。
	LastForwardPeerAddr string `json:"lastForwardPeerAddr,omitempty"`
}

// TicketKeyStatus 描述当前 relay ticket 签名密钥配置和轮转状态。
type TicketKeyStatus struct {
	// Source 标记密钥来源，例如 dev_default/signing_secret/key_ring。
	Source string `json:"source"`
	// KeyRingID 是当前有效密钥集合的短哈希标识。
	KeyRingID string `json:"keyRingId"`
	// SigningConfigured 表示是否配置了显式签名密钥。
	SigningConfigured bool `json:"signingConfigured"`
	// KeyRingConfigured 表示是否配置了多密钥 key ring。
	KeyRingConfigured bool `json:"keyRingConfigured"`
	// EffectiveKeyCount 是当前参与验签的密钥数量。
	EffectiveKeyCount int `json:"effectiveKeyCount"`
	// RotationReady 表示当前配置是否满足无缝轮转的基本条件。
	RotationReady bool `json:"rotationReady"`
	// AcceptsDevFallback 表示是否仍接受开发默认密钥。
	AcceptsDevFallback bool `json:"acceptsDevFallback"`
}

// Store 是 UDP 中继节点的默认内存状态实现。
type Store struct {
	mu                            sync.RWMutex
	sessions                      map[string]*Session
	sources                       map[string]sourceBinding
	attachCount                   uint64
	participantRefreshCount       uint64
	participantAddressChangeCount uint64
	forwardCount                  uint64
	forwardPeerNotAttachedCount   uint64
	lastRefreshSessionID          string
	lastRefreshParticipantID      string
	lastRefreshAddr               string
	lastForwardSessionID          string
	lastForwardParticipantID      string
	lastForwardSourceAddr         string
	lastForwardPeerID             string
	lastForwardPeerAddr           string
}

type sourceBinding struct {
	SessionID     string
	ParticipantID string
}

func NewStore() *Store {
	return &Store{
		sessions: make(map[string]*Session),
		sources:  make(map[string]sourceBinding),
	}
}

func (s *Store) Attach(addr *net.UDPAddr, participantID string, ticket protocol.RelayTicket, transport string) (*Session, string, error) {
	if ticket.SrcNodeID != "" || ticket.DstNodeID != "" {
		switch participantID {
		case ticket.SrcNodeID:
			if ticket.DstNodeID == "" || (ticket.PeerID != "" && ticket.PeerID != ticket.DstNodeID) {
				return nil, "", ErrTicketInvalid
			}
			ticket.PeerID = ticket.DstNodeID
		case ticket.DstNodeID:
			if ticket.SrcNodeID == "" || (ticket.PeerID != "" && ticket.PeerID != ticket.SrcNodeID) {
				return nil, "", ErrTicketInvalid
			}
			ticket.PeerID = ticket.SrcNodeID
		default:
			return nil, "", ErrTicketInvalid
		}
	}
	if participantID == "" || ticket.SessionID == "" || ticket.PeerID == "" {
		return nil, "", ErrTicketInvalid
	}
	if transport != "" && transport != "udp" && transport != "relay_udp" {
		return nil, "", ErrTransportUnsupported
	}
	if ticket.Path != "" && ticket.Path != "relay_udp" {
		return nil, "", ErrTicketInvalid
	}
	if !validRelayTicketSignature(ticket) {
		return nil, "", ErrTicketInvalid
	}
	if !ticket.ExpiresAt.IsZero() && time.Now().After(ticket.ExpiresAt) {
		return nil, "", ErrTicketInvalid
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredLocked(time.Now())

	sourceKey := addr.String()
	if binding, ok := s.sources[sourceKey]; ok {
		if binding.SessionID != ticket.SessionID || binding.ParticipantID != participantID {
			return nil, "", ErrSourceAlreadyAttached
		}
	}

	session, ok := s.sessions[ticket.SessionID]
	if !ok {
		session = &Session{
			ID:                  ticket.SessionID,
			PeerID:              ticket.PeerID,
			Path:                "relay_udp",
			Participants:        make(map[string]*net.UDPAddr),
			ParticipantLastSeen: make(map[string]time.Time),
			ExpiresAt:           ticket.ExpiresAt,
		}
		s.sessions[ticket.SessionID] = session
	} else if session.ExpiresAt.IsZero() || session.ExpiresAt.Before(ticket.ExpiresAt) {
		session.ExpiresAt = ticket.ExpiresAt
	}
	if session.ParticipantLastSeen == nil {
		session.ParticipantLastSeen = make(map[string]time.Time)
	}
	if previous, ok := session.Participants[participantID]; ok && !sameUDPAddr(previous, addr) {
		s.participantAddressChangeCount++
	}
	session.Participants[participantID] = cloneAddr(addr)
	session.ParticipantLastSeen[participantID] = time.Now()
	s.sources[sourceKey] = sourceBinding{
		SessionID:     ticket.SessionID,
		ParticipantID: participantID,
	}
	s.attachCount++

	peer := ""
	for id := range session.Participants {
		if id != participantID {
			peer = id
			break
		}
	}
	return session, peer, nil
}

func validRelayTicketSignature(ticket protocol.RelayTicket) bool {
	if ticket.Signature == "" {
		return false
	}
	payloads := []string{fmt.Sprintf("%s|%s|%s|%s|%s",
		ticket.TicketID,
		ticket.PeerID,
		ticket.SessionID,
		ticket.Path,
		ticket.ExpiresAt.UTC().Format(time.RFC3339Nano),
	)}
	if ticket.NetworkID != "" && ticket.SrcNodeID != "" && ticket.DstNodeID != "" {
		expiresAt := ticket.ExpiresAtRaw
		if expiresAt == "" && !ticket.ExpiresAt.IsZero() {
			expiresAt = ticket.ExpiresAt.UTC().Format(time.RFC3339)
		}
		payloads = append(payloads, fmt.Sprintf("%s|%s|%s|%s|%s|%s",
			ticket.TicketID,
			ticket.NetworkID,
			ticket.SessionID,
			ticket.SrcNodeID,
			ticket.DstNodeID,
			expiresAt,
		))
	}
	for _, secret := range relayTicketSecrets() {
		for _, payload := range payloads {
			mac := hmac.New(sha256.New, []byte(secret))
			_, _ = mac.Write([]byte(payload))
			want := hex.EncodeToString(mac.Sum(nil))
			if hmac.Equal([]byte(want), []byte(ticket.Signature)) {
				return true
			}
		}
	}
	return false
}

func ticketSecret() string {
	for _, secret := range ticketSecrets() {
		return secret
	}
	return "dev-wire-ticket-secret"
}

func ticketSecrets() []string {
	if value := os.Getenv("SLAN_WIRE_TICKET_SECRETS"); value != "" {
		var out []string
		for _, item := range strings.Split(value, ",") {
			if secret := strings.TrimSpace(item); secret != "" {
				out = append(out, secret)
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	if value := os.Getenv("SLAN_WIRE_TICKET_SECRET"); value != "" {
		return []string{value}
	}
	return []string{"dev-wire-ticket-secret"}
}

func relayTicketSecrets() []string {
	secrets := ticketSecrets()
	if value := strings.TrimSpace(os.Getenv("SLAN_RELAY_TICKET_SECRET")); value != "" {
		for _, secret := range secrets {
			if secret == value {
				return secrets
			}
		}
		return append([]string{value}, secrets...)
	}
	return secrets
}

func ticketKeyRingID(secrets []string) string {
	hash := sha256.New()
	_, _ = hash.Write([]byte("slan-wire-ticket-key-ring-v1"))
	for _, secret := range secrets {
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(secret))
	}
	return hex.EncodeToString(hash.Sum(nil))[:16]
}

func CurrentTicketKeyStatus() TicketKeyStatus {
	secrets := ticketSecrets()
	keyRingConfigured := len(parseTicketSecretList(os.Getenv("SLAN_WIRE_TICKET_SECRETS"))) > 0
	signingConfigured := keyRingConfigured || strings.TrimSpace(os.Getenv("SLAN_WIRE_TICKET_SECRET")) != ""
	source := "dev_default"
	if keyRingConfigured {
		source = "key_ring"
	} else if signingConfigured {
		source = "signing_secret"
	}
	return TicketKeyStatus{
		Source:             source,
		KeyRingID:          ticketKeyRingID(secrets),
		SigningConfigured:  signingConfigured,
		KeyRingConfigured:  keyRingConfigured,
		EffectiveKeyCount:  len(secrets),
		RotationReady:      keyRingConfigured && len(secrets) >= 2,
		AcceptsDevFallback: !signingConfigured,
	}
}

func parseTicketSecretList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	var out []string
	for _, item := range strings.Split(value, ",") {
		if secret := strings.TrimSpace(item); secret != "" {
			out = append(out, secret)
		}
	}
	return out
}

func (s *Store) Forward(addr *net.UDPAddr, sessionID, participantID string, _ []byte) (*net.UDPAddr, string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.pruneExpiredLocked(now)

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, "", ErrSessionNotFound
	}
	bound, ok := session.Participants[participantID]
	if !ok {
		return nil, "", ErrParticipantNotFound
	}
	if !sameUDPAddr(bound, addr) {
		for sourceKey, binding := range s.sources {
			if binding.SessionID == sessionID && binding.ParticipantID == participantID {
				delete(s.sources, sourceKey)
			}
		}
		session.Participants[participantID] = cloneAddr(addr)
		s.sources[addr.String()] = sourceBinding{
			SessionID:     sessionID,
			ParticipantID: participantID,
		}
		s.participantRefreshCount++
		s.participantAddressChangeCount++
		s.lastRefreshSessionID = sessionID
		s.lastRefreshParticipantID = participantID
		s.lastRefreshAddr = addr.String()
	}
	if session.ParticipantLastSeen == nil {
		session.ParticipantLastSeen = make(map[string]time.Time)
	}
	session.ParticipantLastSeen[participantID] = now
	for peerID, peerAddr := range session.Participants {
		if peerID != participantID && participantIsActive(session, peerID, now) {
			atomic.AddUint64(&s.forwardCount, 1)
			s.lastForwardSessionID = sessionID
			s.lastForwardParticipantID = participantID
			s.lastForwardSourceAddr = addr.String()
			s.lastForwardPeerID = peerID
			s.lastForwardPeerAddr = peerAddr.String()
			return copyAddrForRead(peerAddr), peerID, nil
		}
	}
	atomic.AddUint64(&s.forwardPeerNotAttachedCount, 1)
	return nil, "", ErrPeerNotAttached
}

func (s *Store) RefreshParticipant(addr *net.UDPAddr, sessionID, participantID string) error {
	if addr == nil || sessionID == "" || participantID == "" {
		return ErrParticipantNotFound
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.pruneExpiredLocked(now)

	session, ok := s.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	if _, ok := session.Participants[participantID]; !ok {
		return ErrParticipantNotFound
	}
	if previous := session.Participants[participantID]; previous != nil && !sameUDPAddr(previous, addr) {
		s.participantAddressChangeCount++
	}
	for sourceKey, binding := range s.sources {
		if binding.SessionID == sessionID && binding.ParticipantID == participantID {
			delete(s.sources, sourceKey)
		}
	}
	session.Participants[participantID] = cloneAddr(addr)
	if session.ParticipantLastSeen == nil {
		session.ParticipantLastSeen = make(map[string]time.Time)
	}
	session.ParticipantLastSeen[participantID] = now
	s.sources[addr.String()] = sourceBinding{
		SessionID:     sessionID,
		ParticipantID: participantID,
	}
	s.participantRefreshCount++
	s.lastRefreshSessionID = sessionID
	s.lastRefreshParticipantID = participantID
	s.lastRefreshAddr = addr.String()
	return nil
}

func (s *Store) Detach(addr *net.UDPAddr, sessionID, participantID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredLocked(time.Now())

	session, ok := s.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	bound, ok := session.Participants[participantID]
	if !ok || !sameUDPAddr(bound, addr) {
		return ErrParticipantNotFound
	}
	delete(session.Participants, participantID)
	delete(session.ParticipantLastSeen, participantID)
	delete(s.sources, addr.String())
	if len(session.Participants) == 0 {
		delete(s.sessions, sessionID)
	}
	return nil
}

func participantIsActive(session *Session, participantID string, now time.Time) bool {
	lastSeen, ok := session.ParticipantLastSeen[participantID]
	return ok && now.Sub(lastSeen) <= participantIdleTimeout
}

func (s *Store) Session(sessionID string) (SessionView, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredLocked(time.Now())
	session, ok := s.sessions[sessionID]
	if !ok {
		return SessionView{}, false
	}
	return sessionView(session), true
}

func (s *Store) Sessions() []SessionView {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredLocked(time.Now())
	out := make([]SessionView, 0, len(s.sessions))
	for _, session := range s.sessions {
		out = append(out, sessionView(session))
	}
	return out
}

func (s *Store) Metrics() Metrics {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneExpiredLocked(time.Now())
	return Metrics{
		SessionCount:                  len(s.sessions),
		SourceBindingCount:            len(s.sources),
		AttachCount:                   s.attachCount,
		ParticipantRefreshCount:       s.participantRefreshCount,
		ParticipantAddressChangeCount: s.participantAddressChangeCount,
		ForwardCount:                  atomic.LoadUint64(&s.forwardCount),
		ForwardPeerNotAttachedCount:   atomic.LoadUint64(&s.forwardPeerNotAttachedCount),
		LastRefreshSessionID:          s.lastRefreshSessionID,
		LastRefreshParticipantID:      s.lastRefreshParticipantID,
		LastRefreshAddr:               s.lastRefreshAddr,
		LastForwardSessionID:          s.lastForwardSessionID,
		LastForwardParticipantID:      s.lastForwardParticipantID,
		LastForwardSourceAddr:         s.lastForwardSourceAddr,
		LastForwardPeerID:             s.lastForwardPeerID,
		LastForwardPeerAddr:           s.lastForwardPeerAddr,
	}
}

func (s *Store) pruneExpiredLocked(now time.Time) {
	for sessionID, session := range s.sessions {
		if session == nil || session.ExpiresAt.IsZero() || !now.After(session.ExpiresAt) {
			continue
		}
		for participantID := range session.Participants {
			for sourceKey, binding := range s.sources {
				if binding.SessionID == sessionID && binding.ParticipantID == participantID {
					delete(s.sources, sourceKey)
				}
			}
		}
		delete(s.sessions, sessionID)
	}
}

func sessionView(session *Session) SessionView {
	participants := make(map[string]string, len(session.Participants))
	for id, addr := range session.Participants {
		participants[id] = addr.String()
	}
	return SessionView{
		ID:               session.ID,
		PeerID:           session.PeerID,
		Path:             session.Path,
		Participants:     participants,
		ParticipantCount: len(participants),
		ExpiresAt:        session.ExpiresAt,
	}
}

func cloneAddr(addr *net.UDPAddr) *net.UDPAddr {
	if addr == nil {
		return nil
	}
	out := *addr
	if addr.IP != nil {
		out.IP = append([]byte(nil), addr.IP...)
	}
	return &out
}

func copyAddrForRead(addr *net.UDPAddr) *net.UDPAddr {
	if addr == nil {
		return nil
	}
	out := *addr
	return &out
}

func sameUDPAddr(a, b *net.UDPAddr) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Port == b.Port && a.Zone == b.Zone && a.IP.Equal(b.IP)
}
