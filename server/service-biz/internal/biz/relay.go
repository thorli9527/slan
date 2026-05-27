package biz

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"
)

const relayTicketTTL = 15 * time.Minute

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
	sessionID := relaySessionID(networkID, srcNodeID, dstNodeID)
	candidate := chooseRelayCandidate(candidates, preferredEndpointIDs, sessionID)
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
	ticket := RelayTicket{
		TicketID:           "rt-" + ticketID,
		NetworkID:          networkID,
		SessionID:          sessionID,
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

func relaySessionID(networkID, srcNodeID, dstNodeID string) string {
	a := strings.TrimSpace(srcNodeID)
	b := strings.TrimSpace(dstNodeID)
	if b < a {
		a, b = b, a
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(networkID) + "|" + a + "|" + b))
	return "rs-" + hex.EncodeToString(sum[:16])
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
		if strings.HasPrefix(address, "derp://") || strings.HasPrefix(address, "derp+tcp+tls://") {
			transport = "derp_tcp_tls_443"
		}
		if transport != "udp" && transport != "derp_tcp_tls_443" {
			continue
		}
		endpointID := fmt.Sprintf("relay-%s-%d", strings.ReplaceAll(transport, "_", "-"), idx+1)
		out = append(out, RelayCandidate{
			EndpointID: endpointID,
			Transport:  transport,
			Address:    address,
			RegionID:   strings.TrimSpace(os.Getenv("SLAN_RELAY_REGION_ID")),
			ClusterID:  strings.TrimSpace(os.Getenv("SLAN_RELAY_CLUSTER_ID")),
		})
	}
	return out
}

func chooseRelayCandidate(candidates []RelayCandidate, preferredEndpointIDs []string, sessionID string) RelayCandidate {
	if candidate, ok := stableRelayCandidate(candidates, sessionID); ok {
		return candidate
	}
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

func stableRelayCandidate(candidates []RelayCandidate, sessionID string) (RelayCandidate, bool) {
	if len(candidates) == 0 {
		return RelayCandidate{}, false
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return RelayCandidate{}, false
	}
	sum := sha256.Sum256([]byte(sessionID))
	index := int(binary.BigEndian.Uint64(sum[:8]) % uint64(len(candidates)))
	return candidates[index], true
}

func relayURL(candidate RelayCandidate) string {
	address := strings.TrimSpace(candidate.Address)
	if strings.Contains(address, "://") {
		return address
	}
	if strings.TrimSpace(candidate.Transport) == "derp_tcp_tls_443" {
		return "derp://" + address
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
