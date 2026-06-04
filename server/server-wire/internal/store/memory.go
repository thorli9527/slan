package store

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/slan/server/server-wire/internal/model"
)

var ErrPeerNotFound = errors.New("peer not found")

type MemoryStore struct {
	mu      sync.RWMutex
	peers   map[string]model.PeerRecord
	derpMap model.DerpMap
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		peers:   make(map[string]model.PeerRecord),
		derpMap: defaultDerpMap(),
	}
}

func (s *MemoryStore) RegisterPeer(reg model.PeerRegistration) (model.PeerRecord, error) {
	if reg.PeerID == "" {
		return model.PeerRecord{}, fmt.Errorf("peerId is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UnixMilli()
	current := s.peers[reg.PeerID]
	current.PeerRegistration = reg
	current.UpdatedAt = now
	if len(current.Endpoints) == 0 {
		current.Endpoints = append([]model.Endpoint(nil), reg.Endpoints...)
	}
	s.peers[reg.PeerID] = current
	return clonePeer(current), nil
}

func (s *MemoryStore) UpdateEndpoints(peerID string, endpoints []model.Endpoint) (model.PeerRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.peers[peerID]
	if !ok {
		return model.PeerRecord{}, ErrPeerNotFound
	}
	current.EndpointChanged = true
	current.RequireMtuRefresh = true
	current.Endpoints = append([]model.Endpoint(nil), endpoints...)
	current.UpdatedAt = time.Now().UnixMilli()
	s.peers[peerID] = current
	return clonePeer(current), nil
}

func (s *MemoryStore) DerpMap() model.DerpMap {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return cloneDerpMap(s.derpMap)
}

func (s *MemoryStore) IssueDerpTicket(peerID, regionID, nodeID string, ttl time.Duration, renewAfter time.Duration) (model.DerpTicket, error) {
	s.mu.RLock()
	current, ok := s.peers[peerID]
	derpMap := cloneDerpMap(s.derpMap)
	s.mu.RUnlock()
	if !ok {
		return model.DerpTicket{}, ErrPeerNotFound
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if regionID == "" {
		regionID = derpMap.PreferredRegionID
	}
	node := model.DerpNode{RegionID: regionID, NodeID: nodeID}
	if regionID == "" || nodeID == "" {
		found := findDerpNode(derpMap, regionID, nodeID)
		if found == nil {
			return model.DerpTicket{}, fmt.Errorf("derp node not found")
		}
		node = *found
	}
	expiresAt := time.Now().Add(ttl)
	ticket := model.DerpTicket{
		TicketID:     randomID("derp"),
		PeerID:       peerID,
		NetworkID:    current.NetworkID,
		Path:         model.PathDerpTCP443,
		RegionID:     node.RegionID,
		NodeID:       node.NodeID,
		ExpiresAt:    expiresAt,
		ExpiresInMs:  ttl.Milliseconds(),
		RenewAfterMs: renewAfter.Milliseconds(),
	}
	ticket.Signature = signDerpTicket(ticket)
	return ticket, nil
}

func (s *MemoryStore) UpdatePathHealth(peerID string, probes []model.PathProbe) (model.PeerRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.peers[peerID]
	if !ok {
		return model.PeerRecord{}, ErrPeerNotFound
	}
	current.Probes = append([]model.PathProbe(nil), probes...)
	current.UpdatedAt = time.Now().UnixMilli()
	s.peers[peerID] = current
	return clonePeer(current), nil
}

func (s *MemoryStore) UpdateDerpHealth(peerID string, samples []model.DerpHealthSample) (model.PeerRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.peers[peerID]
	if !ok {
		return model.PeerRecord{}, ErrPeerNotFound
	}
	current.DerpHealth = append([]model.DerpHealthSample(nil), samples...)
	current.UpdatedAt = time.Now().UnixMilli()
	s.peers[peerID] = current
	return clonePeer(current), nil
}

func (s *MemoryStore) UpdateActivePath(peerID string, path model.PathKind) (model.PeerRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.peers[peerID]
	if !ok {
		return model.PeerRecord{}, ErrPeerNotFound
	}
	if current.ActivePath != "" && current.ActivePath != path {
		if pathRank(path) > pathRank(current.ActivePath) {
			current.RecentPathDowngrades++
		} else {
			current.RecentPathUpgrades++
		}
	}
	current.ActivePath = path
	current.EndpointChanged = false
	current.RequireMtuRefresh = false
	current.UpdatedAt = time.Now().UnixMilli()
	s.peers[peerID] = current
	return clonePeer(current), nil
}

func pathRank(path model.PathKind) int {
	switch path {
	case model.PathLANUDP, model.PathIPv6UDP, model.PathDirectUDP:
		return 0
	case model.PathRelayUDP:
		return 1
	case model.PathDerpTCP443:
		return 2
	default:
		return 1
	}
}

func (s *MemoryStore) IssueRelayTicket(peerID string, relay model.RelayNode, ttl time.Duration, renewAfter time.Duration) (model.RelayTicket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, ok := s.peers[peerID]
	if !ok {
		return model.RelayTicket{}, ErrPeerNotFound
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	expiresAt := time.Now().Add(ttl)
	ticket := model.RelayTicket{
		TicketID:     randomID("relay"),
		PeerID:       peerID,
		SessionID:    randomID("relay-session"),
		Path:         model.PathRelayUDP,
		RegionID:     relay.RegionID,
		NodeID:       relay.NodeID,
		Host:         relay.Host,
		UDPPort:      relay.UDPPort,
		Present:      true,
		ExpiresAt:    expiresAt,
		ExpiresInMs:  ttl.Milliseconds(),
		RenewAfterMs: renewAfter.Milliseconds(),
	}
	ticket.Signature = signRelayTicket(ticket)
	current.RelayTicket = ticket
	current.UpdatedAt = time.Now().UnixMilli()
	s.peers[peerID] = current
	return ticket, nil
}

func signRelayTicket(ticket model.RelayTicket) string {
	payload := fmt.Sprintf("%s|%s|%s|%s|%s",
		ticket.TicketID,
		ticket.PeerID,
		ticket.SessionID,
		ticket.Path,
		ticket.ExpiresAt.UTC().Format(time.RFC3339Nano),
	)
	return hmacHex(payload)
}

func signDerpTicket(ticket model.DerpTicket) string {
	payload := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s",
		ticket.TicketID,
		ticket.PeerID,
		ticket.NetworkID,
		ticket.Path,
		ticket.RegionID,
		ticket.NodeID,
		ticket.ExpiresAt.UTC().Format(time.RFC3339Nano),
	)
	return hmacHex(payload)
}

func hmacHex(payload string) string {
	mac := hmac.New(sha256.New, []byte(ticketSecret()))
	_, _ = mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
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

func TicketKeyStatus() model.TicketKeyStatus {
	secrets := ticketSecrets()
	keyRingConfigured := len(parseTicketSecretList(os.Getenv("SLAN_WIRE_TICKET_SECRETS"))) > 0
	signingConfigured := keyRingConfigured || strings.TrimSpace(os.Getenv("SLAN_WIRE_TICKET_SECRET")) != ""
	source := "dev_default"
	if keyRingConfigured {
		source = "key_ring"
	} else if signingConfigured {
		source = "signing_secret"
	}
	return model.TicketKeyStatus{
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

func (s *MemoryStore) GetPeer(peerID string) (model.PeerRecord, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	current, ok := s.peers[peerID]
	if !ok {
		return model.PeerRecord{}, false
	}
	return clonePeer(current), true
}

func (s *MemoryStore) ListPeersByNetwork(networkID string) []model.PeerRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.PeerRecord, 0)
	for _, peer := range s.peers {
		if peer.NetworkID == networkID {
			out = append(out, clonePeer(peer))
		}
	}
	return out
}

func clonePeer(in model.PeerRecord) model.PeerRecord {
	in.Endpoints = append([]model.Endpoint(nil), in.Endpoints...)
	in.VirtualIPs = append([]string(nil), in.VirtualIPs...)
	in.AllowedIPs = append([]string(nil), in.AllowedIPs...)
	in.Probes = append([]model.PathProbe(nil), in.Probes...)
	in.DerpHealth = append([]model.DerpHealthSample(nil), in.DerpHealth...)
	return in
}

func cloneDerpMap(in model.DerpMap) model.DerpMap {
	out := model.DerpMap{
		PreferredRegionID: in.PreferredRegionID,
		Regions:           make([]model.DerpRegion, 0, len(in.Regions)),
	}
	for _, region := range in.Regions {
		copyRegion := model.DerpRegion{
			RegionID: region.RegionID,
			Name:     region.Name,
			Nodes:    append([]model.DerpNode(nil), region.Nodes...),
		}
		out.Regions = append(out.Regions, copyRegion)
	}
	return out
}

func defaultDerpMap() model.DerpMap {
	return model.DerpMap{
		PreferredRegionID: "cn-east",
		Regions: []model.DerpRegion{
			{
				RegionID: "cn-east",
				Name:     "China East",
				Nodes: []model.DerpNode{
					{RegionID: "cn-east", NodeID: "derp-cn-east-1", Host: "derp-cn-east-1.slan.local", Port: 443},
					{RegionID: "cn-east", NodeID: "derp-cn-east-2", Host: "derp-cn-east-2.slan.local", Port: 443},
				},
			},
			{
				RegionID: "global-fallback",
				Name:     "Global Fallback",
				Nodes: []model.DerpNode{
					{RegionID: "global-fallback", NodeID: "derp-global-1", Host: "derp-global-1.slan.local", Port: 443},
				},
			},
		},
	}
}

func findDerpNode(m model.DerpMap, regionID, nodeID string) *model.DerpNode {
	for _, region := range m.Regions {
		if regionID != "" && region.RegionID != regionID {
			continue
		}
		for _, node := range region.Nodes {
			if nodeID == "" || node.NodeID == nodeID {
				copyNode := node
				return &copyNode
			}
		}
	}
	return nil
}
