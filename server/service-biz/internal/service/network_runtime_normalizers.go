package service

import "strings"

func normalizeRelayCandidatesInput(input RelayCandidatesInput) RelayCandidatesInput {
	input.NetworkID = strings.TrimSpace(input.NetworkID)
	input.DeviceID = strings.TrimSpace(input.DeviceID)
	return input
}

func normalizeCreatePunchConnectSessionInput(input CreatePunchConnectSessionInput) CreatePunchConnectSessionInput {
	input.NetworkID = strings.TrimSpace(input.NetworkID)
	input.RequesterNodeID = strings.TrimSpace(input.RequesterNodeID)
	input.PeerNodeID = strings.TrimSpace(input.PeerNodeID)
	return input
}

func normalizeIssueRelayTicketInput(input IssueRelayTicketInput) IssueRelayTicketInput {
	input.NetworkID = strings.TrimSpace(input.NetworkID)
	input.SrcNodeID = strings.TrimSpace(input.SrcNodeID)
	input.DstNodeID = strings.TrimSpace(input.DstNodeID)
	input.DerpClusterID = strings.TrimSpace(input.DerpClusterID)
	input.Reason = strings.TrimSpace(input.Reason)
	input.RelayRegionID = strings.TrimSpace(input.RelayRegionID)
	for i := range input.PreferredDerpNodeIDs {
		input.PreferredDerpNodeIDs[i] = strings.TrimSpace(input.PreferredDerpNodeIDs[i])
	}
	for i := range input.PreferredRelayEndpointIDs {
		input.PreferredRelayEndpointIDs[i] = strings.TrimSpace(input.PreferredRelayEndpointIDs[i])
	}
	return input
}
