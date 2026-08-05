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
	input.PunchNodeID = strings.TrimSpace(input.PunchNodeID)
	return input
}

func normalizeIssueRelayTicketInput(input IssueRelayTicketInput) IssueRelayTicketInput {
	input.NetworkID = strings.TrimSpace(input.NetworkID)
	input.SrcNodeID = strings.TrimSpace(input.SrcNodeID)
	input.DstNodeID = strings.TrimSpace(input.DstNodeID)
	input.RelayEndpointID = strings.TrimSpace(input.RelayEndpointID)
	input.Reason = strings.TrimSpace(input.Reason)
	return input
}
