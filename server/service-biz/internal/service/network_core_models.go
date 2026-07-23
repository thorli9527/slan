package service

type NetworkView struct {
	NetworkID        string `json:"networkId"`
	OwnerID          string `json:"ownerId"`
	Name             string `json:"name"`
	CIDR             string `json:"cidr"`
	IntraGroupPolicy string `json:"intraGroupPolicy"`
	Default          bool   `json:"default"`
	Status           string `json:"status"`
	CreatedAt        int64  `json:"createdAt"`
	UpdatedAt        int64  `json:"updatedAt"`
}

type NetworkSummaryView struct {
	Network          NetworkView `json:"network"`
	IntraGroupPolicy string      `json:"intraGroupPolicy"`
	Default          bool        `json:"default"`
	DeviceCount      int         `json:"deviceCount"`
	MemberCount      int         `json:"memberCount"`
	ZoneName         string      `json:"zoneName"`
}

type NetworkConfigPeerView struct {
	DeviceID   string               `json:"deviceId"`
	OwnerID    string               `json:"ownerId"`
	OwnerEmail string               `json:"ownerEmail"`
	Alias      string               `json:"alias"`
	GlobalIP   string               `json:"globalIp"`
	GlobalName string               `json:"globalName"`
	Status     string               `json:"status"`
	Endpoints  []DeviceEndpointView `json:"endpoints"`
}

type DeviceEndpointView struct {
	Type      string `json:"type"`
	Address   string `json:"address"`
	UpdatedAt int64  `json:"updatedAt"`
}

type NetworkRuntimePathView struct {
	NATType         string `json:"natType"`
	ActivePath      string `json:"activePath"`
	RelayTransport  string `json:"relayTransport"`
	RelayEndpoint   string `json:"relayEndpoint"`
	DerpNodeID      string `json:"derpNodeId"`
	PeerNodeID      string `json:"peerNodeId"`
	PathScore       int64  `json:"pathScore"`
	SignalScore     int    `json:"signalScore"`
	SignalQuality   string `json:"signalQuality"`
	ObservedRttMs   int64  `json:"observedRttMs"`
	PacketLossPpm   int64  `json:"packetLossPpm"`
	RelayMtu        int    `json:"relayMtu"`
	MaxFramePayload int    `json:"maxFramePayload"`
	TicketExpiresAt string `json:"ticketExpiresAt"`
	TicketRenewDue  bool   `json:"ticketRenewDue"`
	PathDowngrades  int64  `json:"pathDowngrades"`
	PathUpgrades    int64  `json:"pathUpgrades"`
	LastPathChange  string `json:"lastPathChange"`
	ObservedAt      int64  `json:"observedAt"`
}

type NetworkDNSConfigView struct {
	Servers                   []string `json:"servers"`
	SearchDomains             []string `json:"searchDomains"`
	SplitDomains              []string `json:"splitDomains"`
	FallbackToSystemResolvers bool     `json:"fallbackToSystemResolvers"`
}

type NetworkConfigView struct {
	Network              NetworkView             `json:"network"`
	ConfigVersion        int64                   `json:"configVersion"`
	DeviceID             string                  `json:"deviceId"`
	NodeID               string                  `json:"nodeId"`
	GlobalIP             string                  `json:"globalIp"`
	PrefixLen            int                     `json:"prefixLen"`
	GlobalName           string                  `json:"globalName"`
	DNS                  NetworkDNSConfigView    `json:"dns"`
	DeviceGroupsByDevice map[string][]string     `json:"deviceGroupsByDevice"`
	RuntimePath          NetworkRuntimePathView  `json:"runtimePath"`
	Peers                []NetworkConfigPeerView `json:"peers"`
	DNSZones             []DNSZoneView           `json:"dnsZones"`
	DNSRecords           []DNSRecordView         `json:"dnsRecords"`
	SecurityGroups       []SecurityGroupView     `json:"securityGroups"`
	SecurityRules        []SecurityRuleView      `json:"securityRules"`
}

type NetworkResolvedConfigView struct {
	Config          NetworkConfigView    `json:"config"`
	RelayCandidates []RelayCandidateView `json:"relayCandidates"`
}
