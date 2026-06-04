package biz

import (
	"net"
	"strconv"
	"strings"
)

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
