package service

import (
	"os"
	"strconv"
	"strings"
)

func normalizeWireToken(token string) string {
	return strings.TrimSpace(token)
}

func normalizedWireExpectedToken() string {
	return strings.TrimSpace(os.Getenv("SLAN_INTERNAL_WIRE_TOKEN"))
}

func normalizeWireNodeID(nodeID string) string {
	return strings.TrimSpace(nodeID)
}

func normalizeWireRegion(region string) string {
	region = strings.TrimSpace(region)
	if region == "" {
		return "default"
	}
	return region
}

func normalizeWireName(nodeID, name, fallback string) string {
	name = strings.TrimSpace(name)
	if name != "" {
		return name
	}
	nodeID = strings.TrimSpace(nodeID)
	if nodeID != "" {
		return nodeID
	}
	return fallback
}

func normalizeWirePeerID(peerID string) string {
	return strings.TrimSpace(peerID)
}

func normalizeWireNetworkID(networkID string) string {
	return strings.TrimSpace(networkID)
}

func normalizeWirePeerPathHealthInput(input WirePeerPathHealthInput) WirePeerPathHealthInput {
	input.PeerID = strings.TrimSpace(input.PeerID)
	probes := make([]WirePathProbeInput, 0, len(input.Probes))
	for _, probe := range input.Probes {
		probe.Path = strings.TrimSpace(probe.Path)
		if probe.Path == "" {
			continue
		}
		if probe.RTTMs < 0 {
			probe.RTTMs = 0
		}
		if probe.LossPPM < 0 {
			probe.LossPPM = 0
		}
		if probe.MTU < 0 {
			probe.MTU = 0
		}
		if probe.ObservedAt < 0 {
			probe.ObservedAt = 0
		}
		probes = append(probes, probe)
	}
	input.Probes = probes
	return input
}

func wireHeartbeatFreshnessSeconds(fallback int64) int64 {
	if value, err := strconv.ParseInt(strings.TrimSpace(os.Getenv("SLAN_WIRE_NODE_HEARTBEAT_FRESHNESS_SECONDS")), 10, 64); err == nil && value > 0 {
		return value
	}
	return fallback
}
