package app

import servicepkg "github.com/slan/service-biz/internal/service"

type createPunchConnectSessionRequest struct {
	NetworkID       string `json:"networkId"`
	RequesterNodeID string `json:"requesterNodeId"`
	PeerNodeID      string `json:"peerNodeId"`
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
		TTLSeconds:      r.TTLSeconds,
	}
}

type issueRelayTicketRequest struct {
	NetworkID                 string   `json:"networkId"`
	SrcNodeID                 string   `json:"srcNodeId"`
	DstNodeID                 string   `json:"dstNodeId"`
	DerpClusterID             string   `json:"derpClusterId"`
	PreferredDerpNodeIDs      []string `json:"preferredDerpNodeIds"`
	PreferredRelayEndpointIDs []string `json:"preferredRelayEndpointIds"`
	Reason                    string   `json:"reason"`
	RelayRegionID             string   `json:"relayRegionId"`
}

func (r issueRelayTicketRequest) toInput() servicepkg.IssueRelayTicketInput {
	return servicepkg.IssueRelayTicketInput{
		NetworkID:                 r.NetworkID,
		SrcNodeID:                 r.SrcNodeID,
		DstNodeID:                 r.DstNodeID,
		DerpClusterID:             r.DerpClusterID,
		PreferredDerpNodeIDs:      r.PreferredDerpNodeIDs,
		PreferredRelayEndpointIDs: r.PreferredRelayEndpointIDs,
		Reason:                    r.Reason,
		RelayRegionID:             r.RelayRegionID,
	}
}
