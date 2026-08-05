package service

import "github.com/slan/service-biz/internal/model"

func newPunchConnectSession(sessionID string, input CreatePunchConnectSessionInput, punchNodeID, endpointAddress string) model.PunchConnectSession {
	return model.PunchConnectSession{
		SessionID:       sessionID,
		NetworkID:       input.NetworkID,
		RequesterNodeID: input.RequesterNodeID,
		PeerNodeID:      input.PeerNodeID,
		PunchNodeID:     punchNodeID,
		Requester: &model.PunchEndpoint{
			NetworkID:    input.NetworkID,
			NodeID:       input.RequesterNodeID,
			EndpointType: "direct_udp",
			Address:      endpointAddress,
			Reflexive:    endpointAddress,
			NATType:      "unknown",
		},
		Peer: &model.PunchEndpoint{
			NetworkID:    input.NetworkID,
			NodeID:       input.PeerNodeID,
			EndpointType: "direct_udp",
			Address:      endpointAddress,
			Reflexive:    endpointAddress,
			NATType:      "unknown",
		},
	}
}

func newRelayTicket(
	ticketID string,
	sessionID string,
	input IssueRelayTicketInput,
	candidate RelayCandidateView,
	expiresAt string,
	sessionKey string,
	signature string,
) model.RelayTicket {
	return model.RelayTicket{
		TicketID:           ticketID,
		NetworkID:          input.NetworkID,
		SessionID:          sessionID,
		SrcNodeID:          input.SrcNodeID,
		DstNodeID:          input.DstNodeID,
		DERPClusterID:      candidate.ClusterID,
		AllowedDERPNodeIDs: []string{candidate.EndpointID},
		RelayURL:           wireRelayURL(candidate.Transport, candidate.Address),
		ExpiresAt:          expiresAt,
		SessionKey:         sessionKey,
		Signature:          signature,
	}
}
