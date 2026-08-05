package model

type RelayNode struct {
	NodeID                   string `json:"nodeId"`
	Name                     string `json:"name"`
	Region                   string `json:"region"`
	Endpoint                 string `json:"endpoint"`
	Transport                string `json:"transport"`
	Priority                 int    `json:"priority"`
	TicketKeySource          string `json:"ticketKeySource"`
	TicketKeyRingID          string `json:"ticketKeyRingId"`
	TicketSigningConfigured  bool   `json:"ticketSigningConfigured"`
	TicketKeyRingConfigured  bool   `json:"ticketKeyRingConfigured"`
	TicketEffectiveKeyCount  int    `json:"ticketEffectiveKeyCount"`
	TicketRotationReady      bool   `json:"ticketRotationReady"`
	TicketAcceptsDevFallback bool   `json:"ticketAcceptsDevFallback"`
	MaxBandwidthMbps         int    `json:"maxBandwidthMbps"`
	MonthlyTrafficGB         int    `json:"monthlyTrafficGb"`
	UsedTrafficGB            int    `json:"usedTrafficGb"`
	MaxSessions              int    `json:"maxSessions"`
	ActiveSessions           int    `json:"activeSessions"`
	Status                   string `json:"status"`
	Health                   string `json:"health"`
	CreatedAt                int64  `json:"createdAt"`
	UpdatedAt                int64  `json:"updatedAt"`
}

type PunchNode struct {
	NodeID         string `json:"nodeId"`
	Name           string `json:"name"`
	Endpoint       string `json:"endpoint"`
	MaxSessions    int    `json:"maxSessions"`
	ActiveSessions int    `json:"activeSessions"`
	Status         string `json:"status"`
	Health         string `json:"health"`
	Priority       int    `json:"priority"`
	CreatedAt      int64  `json:"createdAt"`
	UpdatedAt      int64  `json:"updatedAt"`
}

type PunchEndpoint struct {
	NetworkID    string `json:"networkId"`
	NodeID       string `json:"nodeId"`
	EndpointType string `json:"type"`
	Address      string `json:"address"`
	Reflexive    string `json:"reflexive"`
	NATType      string `json:"natType"`
}

type PunchConnectSession struct {
	SessionID       string         `json:"sessionId"`
	NetworkID       string         `json:"networkId"`
	RequesterNodeID string         `json:"requesterNodeId"`
	PeerNodeID      string         `json:"peerNodeId"`
	PunchNodeID     string         `json:"punchNodeId,omitempty"`
	Requester       *PunchEndpoint `json:"requester,omitempty"`
	Peer            *PunchEndpoint `json:"peer,omitempty"`
}

type RelayTicket struct {
	TicketID           string   `json:"ticketId"`
	NetworkID          string   `json:"networkId"`
	SessionID          string   `json:"sessionId"`
	SrcNodeID          string   `json:"srcNodeId"`
	DstNodeID          string   `json:"dstNodeId"`
	DERPClusterID      string   `json:"derpClusterId,omitempty"`
	AllowedDERPNodeIDs []string `json:"allowedDerpNodeIds,omitempty"`
	RelayURL           string   `json:"relayUrl"`
	ExpiresAt          string   `json:"expiresAt"`
	SessionKey         string   `json:"sessionKey"`
	Signature          string   `json:"signature"`
}
