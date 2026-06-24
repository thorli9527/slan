package service

type RelayCandidatesInput struct {
	NetworkID string `json:"networkId"`
	DeviceID  string `json:"deviceId"`
}

type CreatePunchConnectSessionInput struct {
	NetworkID       string `json:"networkId"`
	RequesterNodeID string `json:"requesterNodeId"`
	PeerNodeID      string `json:"peerNodeId"`
	TTLSeconds      uint32 `json:"ttlSeconds"`
}

type IssueRelayTicketInput struct {
	NetworkID                 string   `json:"networkId"`
	SrcNodeID                 string   `json:"srcNodeId"`
	DstNodeID                 string   `json:"dstNodeId"`
	DerpClusterID             string   `json:"derpClusterId"`
	PreferredDerpNodeIDs      []string `json:"preferredDerpNodeIds"`
	PreferredRelayEndpointIDs []string `json:"preferredRelayEndpointIds"`
	Reason                    string   `json:"reason"`
	RelayRegionID             string   `json:"relayRegionId"`
}
