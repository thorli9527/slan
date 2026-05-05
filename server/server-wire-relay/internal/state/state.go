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

type Session struct {
	ID           string
	PeerID       string
	Path         string
	Participants map[string]*net.UDPAddr
	ExpiresAt    time.Time
}

type SessionView struct {
	ID               string            `json:"id"`
	PeerID           string            `json:"peerId"`
	Path             string            `json:"path"`
	Participants     map[string]string `json:"participants"`
	ParticipantCount int               `json:"participantCount"`
	ExpiresAt        time.Time         `json:"expiresAt,omitempty"`
}

type Metrics struct {
	SessionCount       int `json:"sessionCount"`
	SourceBindingCount int `json:"sourceBindingCount"`
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

type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	sources  map[string]sourceBinding
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

	sourceKey := addr.String()
	if binding, ok := s.sources[sourceKey]; ok {
		if binding.SessionID != ticket.SessionID || binding.ParticipantID != participantID {
			return nil, "", ErrSourceAlreadyAttached
		}
	}

	session, ok := s.sessions[ticket.SessionID]
	if !ok {
		session = &Session{
			ID:           ticket.SessionID,
			PeerID:       ticket.PeerID,
			Path:         "relay_udp",
			Participants: make(map[string]*net.UDPAddr),
			ExpiresAt:    ticket.ExpiresAt,
		}
		s.sessions[ticket.SessionID] = session
	}
	session.Participants[participantID] = cloneAddr(addr)
	s.sources[sourceKey] = sourceBinding{
		SessionID:     ticket.SessionID,
		ParticipantID: participantID,
	}

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
	payload := fmt.Sprintf("%s|%s|%s|%s|%s",
		ticket.TicketID,
		ticket.PeerID,
		ticket.SessionID,
		ticket.Path,
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

func (s *Store) Forward(addr *net.UDPAddr, sessionID, participantID string, _ []byte) (*net.UDPAddr, string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return nil, "", ErrSessionNotFound
	}
	bound, ok := session.Participants[participantID]
	if !ok {
		return nil, "", ErrParticipantNotFound
	}
	if bound.String() != addr.String() {
		return nil, "", ErrParticipantNotFound
	}
	for peerID, peerAddr := range session.Participants {
		if peerID != participantID {
			return cloneAddr(peerAddr), peerID, nil
		}
	}
	return nil, "", ErrPeerNotAttached
}

func (s *Store) Detach(addr *net.UDPAddr, sessionID, participantID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[sessionID]
	if !ok {
		return ErrSessionNotFound
	}
	bound, ok := session.Participants[participantID]
	if !ok || bound.String() != addr.String() {
		return ErrParticipantNotFound
	}
	delete(session.Participants, participantID)
	delete(s.sources, addr.String())
	if len(session.Participants) == 0 {
		delete(s.sessions, sessionID)
	}
	return nil
}

func (s *Store) Session(sessionID string) (SessionView, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionID]
	if !ok {
		return SessionView{}, false
	}
	return sessionView(session), true
}

func (s *Store) Sessions() []SessionView {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]SessionView, 0, len(s.sessions))
	for _, session := range s.sessions {
		out = append(out, sessionView(session))
	}
	return out
}

func (s *Store) Metrics() Metrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Metrics{
		SessionCount:       len(s.sessions),
		SourceBindingCount: len(s.sources),
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
