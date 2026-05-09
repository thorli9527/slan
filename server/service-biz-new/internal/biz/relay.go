package biz

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"
)

const relayTicketTTL = 10 * time.Minute

func (s *Store) RelayCandidates(networkID, deviceID string) ([]RelayCandidate, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.networkConfigLocked(networkID, deviceID); err != nil {
		return nil, err
	}
	candidates := s.activeRelayCandidatesLocked()
	if len(candidates) == 0 {
		candidates = configuredRelayCandidates()
	}
	return candidates, nil
}

func (s *Store) IssueRelayTicket(networkID, srcNodeID, dstNodeID, derpClusterID string, preferredEndpointIDs []string) (RelayTicket, error) {
	networkID = strings.TrimSpace(networkID)
	srcNodeID = strings.TrimSpace(srcNodeID)
	dstNodeID = strings.TrimSpace(dstNodeID)
	srcDeviceID := deviceIDFromNodeID(srcNodeID)
	dstDeviceID := deviceIDFromNodeID(dstNodeID)
	if networkID == "" || srcDeviceID == "" || dstDeviceID == "" {
		return RelayTicket{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.networkConfigLocked(networkID, srcDeviceID); err != nil {
		return RelayTicket{}, err
	}
	if _, err := s.networkConfigLocked(networkID, dstDeviceID); err != nil {
		return RelayTicket{}, err
	}
	candidates := s.activeRelayCandidatesLocked()
	if len(candidates) == 0 {
		candidates = configuredRelayCandidates()
	}
	if len(candidates) == 0 {
		return RelayTicket{}, errNotFound
	}
	candidate := chooseRelayCandidate(candidates, preferredEndpointIDs)
	now := time.Now().UTC()
	expiresAt := now.Add(relayTicketTTL).Format(time.RFC3339)
	sessionKey, err := secureTokenHex(32)
	if err != nil {
		return RelayTicket{}, err
	}
	ticketID, err := secureTokenHex(16)
	if err != nil {
		return RelayTicket{}, err
	}
	sessionID, err := secureTokenHex(16)
	if err != nil {
		return RelayTicket{}, err
	}
	ticket := RelayTicket{
		TicketID:           "rt-" + ticketID,
		NetworkID:          networkID,
		SessionID:          "rs-" + sessionID,
		SrcNodeID:          srcNodeID,
		DstNodeID:          dstNodeID,
		DERPClusterID:      derpClusterID,
		CountryCode:        candidate.CountryCode,
		AllowedDERPNodeIDs: []string{candidate.EndpointID},
		RelayURL:           relayURL(candidate),
		ExpiresAt:          expiresAt,
		SessionKey:         sessionKey,
	}
	ticket.Signature = signRelayTicket(ticket)
	return ticket, nil
}

func configuredRelayCandidates() []RelayCandidate {
	raw := strings.TrimSpace(os.Getenv("SLAN_RELAY_ENDPOINTS"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("SLAN_RELAY_UDP_ADDR"))
	}
	if raw == "" {
		raw = "udp://127.0.0.1:3478"
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ';' || r == '\n' })
	out := make([]RelayCandidate, 0, len(parts))
	for idx, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		transport := "udp"
		address := value
		if before, after, ok := strings.Cut(value, "="); ok {
			transport = strings.TrimSpace(before)
			address = strings.TrimSpace(after)
		}
		if strings.HasPrefix(address, "udp://") {
			transport = "udp"
		}
		if transport != "udp" {
			continue
		}
		endpointID := fmt.Sprintf("relay-udp-%d", idx+1)
		out = append(out, RelayCandidate{
			EndpointID: endpointID,
			Transport:  "udp",
			Address:    address,
			RegionID:   strings.TrimSpace(os.Getenv("SLAN_RELAY_REGION_ID")),
			ClusterID:  strings.TrimSpace(os.Getenv("SLAN_RELAY_CLUSTER_ID")),
		})
	}
	return out
}

func chooseRelayCandidate(candidates []RelayCandidate, preferredEndpointIDs []string) RelayCandidate {
	for _, preferred := range preferredEndpointIDs {
		preferred = strings.TrimSpace(preferred)
		if preferred == "" {
			continue
		}
		for _, candidate := range candidates {
			if candidate.EndpointID == preferred {
				return candidate
			}
		}
	}
	return candidates[0]
}

func relayURL(candidate RelayCandidate) string {
	address := strings.TrimSpace(candidate.Address)
	if strings.Contains(address, "://") {
		return address
	}
	return "udp://" + address
}

func deviceIDFromNodeID(nodeID string) string {
	nodeID = strings.TrimSpace(nodeID)
	if strings.HasPrefix(nodeID, "node-") {
		return strings.TrimPrefix(nodeID, "node-")
	}
	return nodeID
}

func secureTokenHex(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func signRelayTicket(ticket RelayTicket) string {
	secret := os.Getenv("SLAN_RELAY_TICKET_SECRET")
	if secret == "" {
		secret = os.Getenv("SLAN_MQTT_PASSWORD_SECRET")
	}
	if secret == "" {
		secret = "dev-relay-ticket-secret"
	}
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%s|%s|%s|%s|%s|%s", ticket.TicketID, ticket.NetworkID, ticket.SessionID, ticket.SrcNodeID, ticket.DstNodeID, ticket.ExpiresAt)
	return hex.EncodeToString(mac.Sum(nil))
}
