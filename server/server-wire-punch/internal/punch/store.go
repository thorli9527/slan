package punch

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"time"
)

type Endpoint struct {
	NetworkID    string    `json:"networkId"`
	NodeID       string    `json:"nodeId"`
	Type         string    `json:"type"`
	Address      string    `json:"address"`
	Reflexive    string    `json:"reflexive"`
	NATType      string    `json:"natType"`
	UpdatedAt    time.Time `json:"updatedAt"`
	ExpiresAt    time.Time `json:"expiresAt"`
	UserAgent    string    `json:"userAgent,omitempty"`
	ObservedFrom string    `json:"observedFrom,omitempty"`
}

type ConnectSession struct {
	SessionID       string    `json:"sessionId"`
	NetworkID       string    `json:"networkId"`
	RequesterNodeID string    `json:"requesterNodeId"`
	PeerNodeID      string    `json:"peerNodeId"`
	Requester       *Endpoint `json:"requester,omitempty"`
	Peer            *Endpoint `json:"peer,omitempty"`
	ExpiresAt       time.Time `json:"expiresAt"`
	CreatedAt       time.Time `json:"createdAt"`
}

type Store struct {
	mu        sync.Mutex
	endpoints map[string]Endpoint
	sessions  map[string]ConnectSession
}

type Stats struct {
	EndpointCount int       `json:"endpointCount"`
	SessionCount  int       `json:"sessionCount"`
	UpdatedAt     time.Time `json:"updatedAt"`
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
