package state

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/slan/server/server-wire-derp/internal/protocol"
)

var (
	ErrTicketInvalid   = errors.New("invalid derp ticket")
	ErrSessionNotFound = errors.New("derp session not found")
	ErrPeerNotFound    = errors.New("peer connection not found")
)

type Connection struct {
	PeerID      string
	NodeID      string
	RegionID    string
	RemoteAddr  string
	ConnectedAt time.Time
	LastSeenAt  time.Time
}

type Session struct {
	SessionID string
	PeerA     string
	PeerB     string
	RegionID  string
	NodeID    string
	ExpiresAt time.Time
}

type ConnectionView struct {
	PeerID      string    `json:"peerId"`
	NodeID      string    `json:"nodeId"`
	RegionID    string    `json:"regionId"`
	RemoteAddr  string    `json:"remoteAddr"`
	ConnectedAt time.Time `json:"connectedAt"`
	LastSeenAt  time.Time `json:"lastSeenAt"`
}

type SessionView struct {
	SessionID string    `json:"sessionId"`
	PeerA     string    `json:"peerA"`
	PeerB     string    `json:"peerB,omitempty"`
	RegionID  string    `json:"regionId"`
	NodeID    string    `json:"nodeId"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type Metrics struct {
	ConnectionCount    int `json:"connectionCount"`
	SessionCount       int `json:"sessionCount"`
	RegionHealthyCount int `json:"regionHealthyCount"`
	NodeHealthyCount   int `json:"nodeHealthyCount"`
}

type TicketKeyStatus struct {
	Source             string `json:"source"`
	KeyRingID          string `json:"keyRingId"`
	SigningConfigured  bool   `json:"signingConfigured"`
	KeyRingConfigured  bool   `json:"keyRingConfigured"`
	EffectiveKeyCount  int    `json:"effectiveKeyCount"`
	RotationReady      bool   `json:"rotationReady"`
	AcceptsDevFallback bool   `json:"acceptsDevFallback"`
}

type RegionView struct {
	RegionID        string   `json:"regionId"`
	NodeIDs         []string `json:"nodeIds"`
	Healthy         bool     `json:"healthy"`
	ConnectionCount int      `json:"connectionCount"`
}

type Store struct {
	mu          sync.RWMutex
	connections map[string]Connection
	sessions    map[string]Session
}

func NewStore() *Store {
	return &Store{
		connections: make(map[string]Connection),
		sessions:    make(map[string]Session),
	}
}

func (s *Store) Connect(conn net.Conn, peerID, nodeID, regionID string, ticket protocol.DerpTicket) (Session, int64, error) {
	if peerID == "" || ticket.PeerID == "" || ticket.Path != "derp_tcp_tls_443" || ticket.NodeID == "" || ticket.RegionID == "" {
		return Session{}, 0, ErrTicketInvalid
	}
	if peerID != ticket.PeerID {
		return Session{}, 0, ErrTicketInvalid
	}
	if !validDerpTicketSignature(ticket) {
		return Session{}, 0, ErrTicketInvalid
	}
	if !ticket.ExpiresAt.IsZero() && time.Now().After(ticket.ExpiresAt) {
		return Session{}, 0, ErrTicketInvalid
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	s.connections[peerID] = Connection{
		PeerID:      peerID,
		NodeID:      firstNonEmpty(nodeID, ticket.NodeID),
		RegionID:    firstNonEmpty(regionID, ticket.RegionID),
		RemoteAddr:  conn.RemoteAddr().String(),
		ConnectedAt: now,
		LastSeenAt:  now,
	}

	session := Session{
		SessionID: randomSessionID(),
		PeerA:     peerID,
		RegionID:  firstNonEmpty(regionID, ticket.RegionID),
		NodeID:    firstNonEmpty(nodeID, ticket.NodeID),
		ExpiresAt: ticket.ExpiresAt,
	}
	s.sessions[session.SessionID] = session
	return session, time.Until(ticket.ExpiresAt).Milliseconds() / 2, nil
}

func randomSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("generate derp session id: %v", err))
	}
	return "derp-session-" + hex.EncodeToString(b[:])
}

func validDerpTicketSignature(ticket protocol.DerpTicket) bool {
	if ticket.Signature == "" {
		return false
	}
	payload := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s",
		ticket.TicketID,
		ticket.PeerID,
		ticket.NetworkID,
		ticket.Path,
		ticket.RegionID,
		ticket.NodeID,
		ticket.ExpiresAt.UTC().Format(time.RFC3339Nano),
	)
	for _, secret := range ticketSecrets() {
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write([]byte(payload))
		want := hex.EncodeToString(mac.Sum(nil))
		if hmac.Equal([]byte(want), []byte(ticket.Signature)) {
			return true
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

func (s *Store) TouchPeer(peerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.connections[peerID]
	if !ok {
		return
	}
	current.LastSeenAt = time.Now()
	s.connections[peerID] = current
}

func (s *Store) BindSessionPeer(sessionID, targetPeerID string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	if targetPeerID != "" && session.PeerA != targetPeerID {
		session.PeerB = targetPeerID
		s.sessions[sessionID] = session
	}
	return session, nil
}

func (s *Store) Disconnect(peerID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.connections, peerID)
	for id, session := range s.sessions {
		if session.PeerA == peerID || session.PeerB == peerID {
			delete(s.sessions, id)
		}
	}
}

func (s *Store) Connection(peerID string) (ConnectionView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.connections[peerID]
	if !ok {
		return ConnectionView{}, false
	}
	return toConnectionView(value), true
}

func (s *Store) Connections() []ConnectionView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]ConnectionView, 0, len(s.connections))
	for _, value := range s.connections {
		out = append(out, toConnectionView(value))
	}
	return out
}

func (s *Store) Session(sessionID string) (SessionView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.sessions[sessionID]
	if !ok {
		return SessionView{}, false
	}
	return toSessionView(value), true
}

func (s *Store) Sessions() []SessionView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SessionView, 0, len(s.sessions))
	for _, value := range s.sessions {
		out = append(out, toSessionView(value))
	}
	return out
}

func (s *Store) Metrics() Metrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	regions := map[string]struct{}{}
	nodes := map[string]struct{}{}
	for _, conn := range s.connections {
		if conn.RegionID != "" {
			regions[conn.RegionID] = struct{}{}
		}
		if conn.NodeID != "" {
			nodes[conn.NodeID] = struct{}{}
		}
	}
	return Metrics{
		ConnectionCount:    len(s.connections),
		SessionCount:       len(s.sessions),
		RegionHealthyCount: len(regions),
		NodeHealthyCount:   len(nodes),
	}
}

func (s *Store) Regions() []RegionView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	type agg struct {
		nodes map[string]struct{}
		count int
	}
	aggs := map[string]*agg{}
	for _, conn := range s.connections {
		if conn.RegionID == "" {
			continue
		}
		item := aggs[conn.RegionID]
		if item == nil {
			item = &agg{nodes: map[string]struct{}{}}
			aggs[conn.RegionID] = item
		}
		item.count++
		if conn.NodeID != "" {
			item.nodes[conn.NodeID] = struct{}{}
		}
	}
	out := make([]RegionView, 0, len(aggs))
	for regionID, item := range aggs {
		nodeIDs := make([]string, 0, len(item.nodes))
		for nodeID := range item.nodes {
			nodeIDs = append(nodeIDs, nodeID)
		}
		out = append(out, RegionView{
			RegionID:        regionID,
			NodeIDs:         nodeIDs,
			Healthy:         item.count > 0,
			ConnectionCount: item.count,
		})
	}
	return out
}

func toConnectionView(value Connection) ConnectionView {
	return ConnectionView(value)
}

func toSessionView(value Session) SessionView {
	return SessionView(value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
