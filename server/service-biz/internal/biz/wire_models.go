package biz

type wireTicketKeyStatus struct {
	Source             string `json:"source,omitempty"`
	KeyRingID          string `json:"keyRingId,omitempty"`
	SigningConfigured  bool   `json:"signingConfigured"`
	KeyRingConfigured  bool   `json:"keyRingConfigured"`
	EffectiveKeyCount  int    `json:"effectiveKeyCount,omitempty"`
	RotationReady      bool   `json:"rotationReady"`
	AcceptsDevFallback bool   `json:"acceptsDevFallback,omitempty"`
}

type wireRelayNodeRequest struct {
	RegionID          string              `json:"regionId"`
	NodeID            string              `json:"nodeId"`
	Host              string              `json:"host"`
	UDPPort           int                 `json:"udpPort"`
	AdminPort         int                 `json:"adminPort,omitempty"`
	Enabled           *bool               `json:"enabled,omitempty"`
	Healthy           *bool               `json:"healthy,omitempty"`
	Priority          int                 `json:"priority,omitempty"`
	TicketKeyRotation wireTicketKeyStatus `json:"ticketKeyRotation,omitempty"`
}

type wireDerpNodeRequest struct {
	RegionID          string              `json:"regionId"`
	NodeID            string              `json:"nodeId"`
	Name              string              `json:"name,omitempty"`
	Host              string              `json:"host"`
	Port              int                 `json:"port,omitempty"`
	Enabled           *bool               `json:"enabled,omitempty"`
	Healthy           *bool               `json:"healthy,omitempty"`
	Priority          int                 `json:"priority,omitempty"`
	TicketKeyRotation wireTicketKeyStatus `json:"ticketKeyRotation,omitempty"`
}

type wireNodeHeartbeatRequest struct {
	Healthy           bool                `json:"healthy"`
	TicketKeyRotation wireTicketKeyStatus `json:"ticketKeyRotation,omitempty"`
}

type wireNodeStatusRequest struct {
	Enabled *bool `json:"enabled,omitempty"`
	Healthy *bool `json:"healthy,omitempty"`
}

type wireEndpoint struct {
	Kind       string `json:"kind"`
	Address    string `json:"address"`
	Port       int    `json:"port,omitempty"`
	Reachable  bool   `json:"reachable,omitempty"`
	ObservedAt int64  `json:"observedAt,omitempty"`
}

type wirePeerRecord struct {
	PeerID                  string         `json:"peerId"`
	NetworkID               string         `json:"networkId,omitempty"`
	NodeID                  string         `json:"nodeId,omitempty"`
	PublicKey               string         `json:"publicKey,omitempty"`
	VirtualIPs              []string       `json:"virtualIps,omitempty"`
	AllowedIPs              []string       `json:"allowedIps,omitempty"`
	SupportsLanDirect       bool           `json:"supportsLanDirect"`
	SupportsIpv6Direct      bool           `json:"supportsIpv6Direct"`
	SupportsDirectUdp       bool           `json:"supportsDirectUdp"`
	SupportsRelayUdp        bool           `json:"supportsRelayUdp"`
	SupportsDerpTcpTls443   bool           `json:"supportsDerpTcpTls443"`
	PreferIpv6              bool           `json:"preferIpv6"`
	PreferLan               bool           `json:"preferLan"`
	AllowEndpointRoaming    bool           `json:"allowEndpointRoaming"`
	AllowFastReselection    bool           `json:"allowFastReselection"`
	AllowRelayTicketRenewal bool           `json:"allowRelayTicketRenewal"`
	KeepaliveIntervalSecs   int            `json:"keepaliveIntervalSecs,omitempty"`
	Endpoints               []wireEndpoint `json:"endpoints,omitempty"`
	ActivePath              string         `json:"activePath,omitempty"`
	RequireMtuRefresh       bool           `json:"requireMtuRefresh"`
	EndpointChanged         bool           `json:"endpointChanged"`
	UpdatedAt               int64          `json:"updatedAt"`
}

type wirePeerContext struct {
	networkDevice NetworkDevice
	device        Device
	runtime       DeviceRuntimeStatus
}
