package service

import "strings"

func normalizeUpsertRuntimeNodeInput(input UpsertRuntimeNodeInput) UpsertRuntimeNodeInput {
	input.NodeID = strings.TrimSpace(input.NodeID)
	input.Name = strings.TrimSpace(input.Name)
	input.DERPRegionID = strings.TrimSpace(input.DERPRegionID)
	input.Endpoint = normalizeNodeEndpoint(input.Endpoint)
	input.Transport = strings.TrimSpace(input.Transport)
	input.Status = strings.TrimSpace(input.Status)
	input.Health = strings.TrimSpace(input.Health)
	return input
}

func normalizeNodeID(nodeID string) string {
	return strings.TrimSpace(nodeID)
}

func normalizeNodeStatus(status string) string {
	return strings.TrimSpace(status)
}

func normalizeNodeEndpoint(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	for _, prefix := range []string{"udp://", "derp://", "derp+tcp+tls://"} {
		endpoint = strings.TrimPrefix(endpoint, prefix)
	}
	return endpoint
}
