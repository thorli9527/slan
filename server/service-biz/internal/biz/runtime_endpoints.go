package biz

import (
	"net"
	"strconv"
	"strings"
	"time"
)

func (s *Server) deviceSessionResponse(device Device, session DeviceSession, configs []NetworkConfig, now time.Time) DeviceSessionResponse {
	mqtt := deviceMQTTCredential(s.mqtt, device.DeviceID, now)
	return DeviceSessionResponse{
		Device:           device,
		DeviceSession:    session,
		MQTT:             mqtt,
		NetworkConfigs:   ItemsResponse{Items: configs},
		RuntimeEndpoints: s.runtimeEndpointsResponse(mqtt, configs, now),
	}
}

func (s *Server) runtimeEndpointsResponse(mqtt *MQTTCredential, configs []NetworkConfig, now time.Time) RuntimeEndpointsResponse {
	punchNodes := runtimePunchNodes(s.store.ActivePunchNodes())
	if len(punchNodes) == 0 {
		punchNodes = runtimePunchNodes(configuredPunchNodes())
	}
	networks := make([]RuntimeNetworkEndpoint, 0, len(configs))
	relayCandidates := make([]RelayCandidate, 0)
	seen := make(map[string]struct{})
	for _, config := range configs {
		networkCandidates := dedupeRuntimeRelayCandidates(config.RelayCandidates)
		networks = append(networks, RuntimeNetworkEndpoint{
			NetworkID:       config.NetworkID,
			RelayCandidates: networkCandidates,
		})
		for _, candidate := range networkCandidates {
			key := relayCandidateRuntimeKey(candidate)
			if key == "" {
				continue
			}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			relayCandidates = append(relayCandidates, candidate)
		}
	}
	return RuntimeEndpointsResponse{
		MQTT:            mqtt,
		PunchNodes:      punchNodes,
		RelayCandidates: relayCandidates,
		Networks:        networks,
		RefreshedAt:     now.Unix(),
	}
}

func dedupeRuntimeRelayCandidates(candidates []RelayCandidate) []RelayCandidate {
	out := make([]RelayCandidate, 0, len(candidates))
	seen := make(map[string]struct{})
	for _, candidate := range candidates {
		candidate.Address = relayCandidatePublicAddress(candidate.Address)
		key := relayCandidateRuntimeKey(candidate)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

func runtimePunchNodes(nodes []OpsPunchNode) []RuntimePunchNode {
	result := make([]RuntimePunchNode, 0, len(nodes))
	for _, node := range nodes {
		host := strings.TrimSpace(node.PublicUDPIP)
		if host == "" || node.PublicUDPPort <= 0 {
			continue
		}
		result = append(result, RuntimePunchNode{
			NodeID:        strings.TrimSpace(node.NodeID),
			Name:          strings.TrimSpace(node.Name),
			Region:        strings.TrimSpace(node.Region),
			Address:       net.JoinHostPort(host, strconv.Itoa(node.PublicUDPPort)),
			PublicUDPIP:   host,
			PublicUDPPort: node.PublicUDPPort,
		})
	}
	return result
}

func relayCandidateRuntimeKey(candidate RelayCandidate) string {
	transport := strings.TrimSpace(candidate.Transport)
	address := strings.TrimSpace(relayCandidatePublicAddress(candidate.Address))
	if transport == "" || address == "" {
		return ""
	}
	return transport + "|" + address
}
