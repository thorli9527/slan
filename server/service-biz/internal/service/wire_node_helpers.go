package service

import (
	"crypto/sha256"
	"encoding/binary"
	"strings"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/wirekit"
)

func wireNodeStatus(enabled, healthy *bool) string {
	if enabled != nil && !*enabled {
		return "disabled"
	}
	return "active"
}

func applyWireNodeHealth(item *model.RelayNode, enabled, healthy *bool) {
	if item == nil {
		return
	}
	if item.Status == "" {
		item.Status = "active"
	}
	if item.Health == "" {
		item.Health = "healthy"
	}
	if enabled != nil {
		if *enabled {
			item.Status = "active"
			if item.Health == "" {
				item.Health = "healthy"
			}
		} else {
			item.Status = "disabled"
			item.Health = "down"
		}
	}
	if healthy != nil && item.Status != "disabled" {
		if *healthy {
			item.Health = "healthy"
		} else {
			item.Health = "down"
		}
	}
}

func wireNodeStale(updatedAt, now int64) bool {
	freshness := wirekit.FreshnessSeconds()
	freshness = wireHeartbeatFreshnessSeconds(freshness)
	return wirekit.IsNodeStale(updatedAt, now, freshness)
}

func firstPositive(values ...int) int {
	for _, value := range values {
		if value > 0 {
			return value
		}
	}
	return 0
}

func wireCandidateTransport(transport string) string {
	transport = strings.TrimSpace(transport)
	switch transport {
	case "", "relay_udp":
		return "udp"
	default:
		return transport
	}
}

func wireCandidateAddress(address string) string {
	address = strings.TrimSpace(address)
	lower := strings.ToLower(address)
	for _, prefix := range []string{"udp://", "derp://", "derp+tcp+tls://"} {
		if strings.HasPrefix(lower, prefix) {
			return strings.TrimSpace(address[len(prefix):])
		}
	}
	return address
}

func wireRelayURL(transport, address string) string {
	address = strings.TrimSpace(address)
	if strings.Contains(address, "://") {
		return address
	}
	if strings.TrimSpace(transport) == "derp_tcp_tls_443" {
		return "derp://" + address
	}
	return "udp://" + address
}

func wireFilterCandidatesForPreferredTransport(candidates []RelayCandidateView, preferredEndpointIDs []string) []RelayCandidateView {
	preferredTransport := ""
	for _, preferred := range preferredEndpointIDs {
		preferred = strings.TrimSpace(preferred)
		if preferred == "" {
			continue
		}
		for _, candidate := range candidates {
			if candidate.EndpointID == preferred {
				preferredTransport = strings.TrimSpace(candidate.Transport)
				break
			}
		}
		if preferredTransport != "" {
			break
		}
	}
	if preferredTransport == "" {
		return nil
	}
	filtered := make([]RelayCandidateView, 0, len(candidates))
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Transport) == preferredTransport {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func wireStableRelayCandidate(candidates []RelayCandidateView, sessionID string) (RelayCandidateView, bool) {
	if len(candidates) == 0 {
		return RelayCandidateView{}, false
	}
	if len(candidates) == 1 {
		return candidates[0], true
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return RelayCandidateView{}, false
	}
	sum := sha256.Sum256([]byte(sessionID))
	index := int(binary.BigEndian.Uint64(sum[:8]) % uint64(len(candidates)))
	return candidates[index], true
}

func chooseWireRelayCandidate(candidates []RelayCandidateView, preferredEndpointIDs []string, sessionID string) (RelayCandidateView, bool) {
	if filtered := wireFilterCandidatesForPreferredTransport(candidates, preferredEndpointIDs); len(filtered) > 0 {
		if candidate, ok := wireStableRelayCandidate(filtered, sessionID); ok {
			return candidate, true
		}
		return filtered[0], true
	}
	if candidate, ok := wireStableRelayCandidate(candidates, sessionID); ok {
		return candidate, true
	}
	for _, preferred := range preferredEndpointIDs {
		preferred = strings.TrimSpace(preferred)
		if preferred == "" {
			continue
		}
		for _, candidate := range candidates {
			if candidate.EndpointID == preferred {
				return candidate, true
			}
		}
	}
	if len(candidates) == 0 {
		return RelayCandidateView{}, false
	}
	return candidates[0], true
}

func wireTicketKeyStatus(item model.RelayNode) *wirekit.TicketKeyStatus {
	if item.TicketKeySource == "" &&
		item.TicketKeyRingID == "" &&
		!item.TicketSigningConfigured &&
		!item.TicketKeyRingConfigured &&
		item.TicketEffectiveKeyCount == 0 &&
		!item.TicketRotationReady &&
		!item.TicketAcceptsDevFallback {
		return nil
	}
	return &wirekit.TicketKeyStatus{
		Source:             item.TicketKeySource,
		KeyRingID:          item.TicketKeyRingID,
		SigningConfigured:  item.TicketSigningConfigured,
		KeyRingConfigured:  item.TicketKeyRingConfigured,
		EffectiveKeyCount:  item.TicketEffectiveKeyCount,
		RotationReady:      item.TicketRotationReady,
		AcceptsDevFallback: item.TicketAcceptsDevFallback,
	}
}

func applyWireTicketKeyStatus(item *model.RelayNode, status *wirekit.TicketKeyStatus) {
	if item == nil || status == nil {
		return
	}
	item.TicketKeySource = status.Source
	item.TicketKeyRingID = status.KeyRingID
	item.TicketSigningConfigured = status.SigningConfigured
	item.TicketKeyRingConfigured = status.KeyRingConfigured
	item.TicketEffectiveKeyCount = status.EffectiveKeyCount
	item.TicketRotationReady = status.RotationReady
	item.TicketAcceptsDevFallback = status.AcceptsDevFallback
}
