package biz

// RelayCandidate 是 biz 下发给客户端的数据面候选转发节点。
type RelayCandidate struct {
	EndpointID  string `json:"endpointId"`
	Transport   string `json:"transport"`
	Address     string `json:"address"`
	CountryCode string `json:"countryCode,omitempty"`
	RegionID    string `json:"regionId,omitempty"`
	ClusterID   string `json:"clusterId,omitempty"`
}

// RelayTicket 是客户端连接 UDP relay 或 DERP 前必须携带的短期授权票据。
type RelayTicket struct {
	TicketID           string   `json:"ticketId"`
	NetworkID          string   `json:"networkId"`
	SessionID          string   `json:"sessionId"`
	SrcNodeID          string   `json:"srcNodeId"`
	DstNodeID          string   `json:"dstNodeId"`
	DERPClusterID      string   `json:"derpClusterId,omitempty"`
	CountryCode        string   `json:"countryCode,omitempty"`
	CityCode           string   `json:"cityCode,omitempty"`
	AllowedDERPNodeIDs []string `json:"allowedDerpNodeIds"`
	RelayURL           string   `json:"relayUrl"`
	ExpiresAt          string   `json:"expiresAt"`
	SessionKey         string   `json:"sessionKey"`
	Signature          string   `json:"signature"`
}
