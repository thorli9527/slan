package app

import servicepkg "github.com/slan/service-biz/internal/service"

type createPunchConnectSessionRequest struct {
	NetworkID       string `json:"networkId"`
	RequesterNodeID string `json:"requesterNodeId"`
	PeerNodeID      string `json:"peerNodeId"`
	PunchNodeID     string `json:"punchNodeId"`
	TTLSeconds      uint32 `json:"ttlSeconds"`
}

type relayCandidatesRequest struct {
	DeviceID string `json:"deviceId"`
}

func (r relayCandidatesRequest) toInput() servicepkg.RelayCandidatesInput {
	return servicepkg.RelayCandidatesInput{
		DeviceID: r.DeviceID,
	}
}

func (r createPunchConnectSessionRequest) toInput() servicepkg.CreatePunchConnectSessionInput {
	return servicepkg.CreatePunchConnectSessionInput{
		NetworkID:       r.NetworkID,
		RequesterNodeID: r.RequesterNodeID,
		PeerNodeID:      r.PeerNodeID,
		PunchNodeID:     r.PunchNodeID,
		TTLSeconds:      r.TTLSeconds,
	}
}

type issueRelayTicketRequest struct {
	NetworkID       string `json:"networkId"`
	SrcNodeID       string `json:"srcNodeId"`
	DstNodeID       string `json:"dstNodeId"`
	RelayEndpointID string `json:"relayEndpointId"`
	Reason          string `json:"reason"`
}

func (r issueRelayTicketRequest) toInput() servicepkg.IssueRelayTicketInput {
	return servicepkg.IssueRelayTicketInput{
		NetworkID:       r.NetworkID,
		SrcNodeID:       r.SrcNodeID,
		DstNodeID:       r.DstNodeID,
		RelayEndpointID: r.RelayEndpointID,
		Reason:          r.Reason,
	}
}
