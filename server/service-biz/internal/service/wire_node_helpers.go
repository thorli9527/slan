package service

import (
	"crypto/sha256"
	"encoding/hex"
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

func wirePunchNodeHealth(enabled, healthy *bool) string {
	if enabled != nil && !*enabled {
		return "down"
	}
	if healthy != nil && !*healthy {
		return "down"
	}
	return "healthy"
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

func applyWirePunchNodeHealth(item *model.PunchNode, enabled, healthy *bool) {
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

func stableRelaySessionSeed(networkID, srcNodeID, dstNodeID string) string {
	left, right := canonicalRelayNodePair(srcNodeID, dstNodeID)
	return strings.Join([]string{
		strings.TrimSpace(networkID),
		left,
		right,
	}, "|")
}

func stableRelaySessionID(networkID, srcNodeID, dstNodeID string, candidate RelayCandidateView) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		stableRelaySessionSeed(networkID, srcNodeID, dstNodeID),
		strings.TrimSpace(candidate.EndpointID),
		strings.TrimSpace(candidate.Transport),
		strings.TrimSpace(candidate.Address),
	}, "|")))
	return "relay-session-" + hex.EncodeToString(sum[:12])
}

func canonicalRelayNodePair(srcNodeID, dstNodeID string) (string, string) {
	left := strings.TrimSpace(srcNodeID)
	right := strings.TrimSpace(dstNodeID)
	if right < left {
		return right, left
	}
	return left, right
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
