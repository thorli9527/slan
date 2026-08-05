package service

type RelayCandidatesInput struct {
	NetworkID string `json:"networkId"`
	DeviceID  string `json:"deviceId"`
}

type CreatePunchConnectSessionInput struct {
	NetworkID       string `json:"networkId"`
	RequesterNodeID string `json:"requesterNodeId"`
	PeerNodeID      string `json:"peerNodeId"`
	PunchNodeID     string `json:"punchNodeId"`
	TTLSeconds      uint32 `json:"ttlSeconds"`
}

type IssueRelayTicketInput struct {
	NetworkID       string `json:"networkId"`
	SrcNodeID       string `json:"srcNodeId"`
	DstNodeID       string `json:"dstNodeId"`
	RelayEndpointID string `json:"relayEndpointId"`
	Reason          string `json:"reason"`
}
