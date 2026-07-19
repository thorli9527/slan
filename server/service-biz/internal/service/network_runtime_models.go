package service

type RelayCandidateView struct {
	EndpointID    string `json:"endpointId"`
	Transport     string `json:"transport"`
	Address       string `json:"address"`
	CountryCode   string `json:"countryCode"`
	RegionID      string `json:"regionId"`
	ClusterID     string `json:"clusterId"`
	Reachable     bool   `json:"reachable,omitempty"`
	ObservedRttMs int64  `json:"observedRttMs,omitempty"`
	PathScore     int64  `json:"pathScore,omitempty"`
	Selected      bool   `json:"selected,omitempty"`
}

type PunchNodeView struct {
	NodeID        string `json:"nodeId"`
	Name          string `json:"name"`
	Region        string `json:"region"`
	Address       string `json:"address"`
	PublicUDPIP   string `json:"publicUdpIp"`
	PublicUDPPort int    `json:"publicUdpPort"`
	Priority      int    `json:"priority"`
}

type PunchConnectEndpointView struct {
	NetworkID    string `json:"networkId"`
	NodeID       string `json:"nodeId"`
	EndpointType string `json:"endpointType"`
	Address      string `json:"address"`
	Reflexive    string `json:"reflexive"`
	NATType      string `json:"natType"`
}

type PunchConnectSessionView struct {
	SessionID       string                    `json:"sessionId"`
	NetworkID       string                    `json:"networkId"`
	RequesterNodeID string                    `json:"requesterNodeId"`
	PeerNodeID      string                    `json:"peerNodeId"`
	PunchNodeID     string                    `json:"punchNodeId,omitempty"`
	Requester       *PunchConnectEndpointView `json:"requester,omitempty"`
	Peer            *PunchConnectEndpointView `json:"peer,omitempty"`
}

type RelayTicketView struct {
	TicketID           string   `json:"ticketId"`
	NetworkID          string   `json:"networkId"`
	SessionID          string   `json:"sessionId"`
	SrcNodeID          string   `json:"srcNodeId"`
	DstNodeID          string   `json:"dstNodeId"`
	DERPClusterID      string   `json:"derpClusterId"`
	CountryCode        string   `json:"countryCode"`
	CityCode           string   `json:"cityCode"`
	AllowedDERPNodeIDs []string `json:"allowedDerpNodeIds"`
	RelayURL           string   `json:"relayUrl"`
	ExpiresAt          string   `json:"expiresAt"`
	SessionKey         string   `json:"sessionKey"`
	Signature          string   `json:"signature"`
}
