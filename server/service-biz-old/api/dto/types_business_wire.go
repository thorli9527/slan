package dto

// WirePeerAuthzView is the internal authorization view consumed by server-wire.
type WirePeerAuthzView struct {
	PeerID      string   `json:"peerId"`
	DeviceID    string   `json:"deviceId,omitempty"`
	NodeID      string   `json:"nodeId,omitempty"`
	NetworkID   string   `json:"networkId,omitempty"`
	Enabled     bool     `json:"enabled"`
	VirtualIPs  []string `json:"virtualIps,omitempty"`
	AllowedIPs  []string `json:"allowedIps,omitempty"`
	QuotaPolicy string   `json:"quotaPolicy,omitempty"`
}

// WirePeerRuntimeConfigView is the static runtime policy view for one peer.
type WirePeerRuntimeConfigView struct {
	PeerID               string    `json:"peerId"`
	DeviceID             string    `json:"deviceId,omitempty"`
	NodeID               string    `json:"nodeId,omitempty"`
	NetworkID            string    `json:"networkId,omitempty"`
	VirtualIPs           []string  `json:"virtualIps,omitempty"`
	AllowedIPs           []string  `json:"allowedIps,omitempty"`
	DNS                  DNSConfig `json:"dns"`
	DefaultKeepaliveSecs int       `json:"defaultKeepaliveSecs"`
	NetworkEnabled       bool      `json:"networkEnabled"`
}

// WireNetworkTopologyView is the static topology view for one business network.
type WireNetworkTopologyView struct {
	NetworkID string             `json:"networkId"`
	Peers     []WireTopologyPeer `json:"peers,omitempty"`
	DNS       DNSConfig          `json:"dns"`
}

type WireTopologyPeer struct {
	PeerID         string   `json:"peerId"`
	DeviceID       string   `json:"deviceId,omitempty"`
	NodeID         string   `json:"nodeId,omitempty"`
	PublicKey      string   `json:"publicKey,omitempty"`
	VirtualIPs     []string `json:"virtualIps,omitempty"`
	AllowedIPs     []string `json:"allowedIps,omitempty"`
	NetworkEnabled bool     `json:"networkEnabled"`
}

type WireDerpMapView struct {
	PreferredRegionID string           `json:"preferredRegionId,omitempty"`
	Regions           []WireDerpRegion `json:"regions"`
}

type WireDerpRegion struct {
	RegionID string         `json:"regionId"`
	Name     string         `json:"name"`
	Nodes    []WireDerpNode `json:"nodes"`
}

type WireDerpNode struct {
	RegionID string `json:"regionId"`
	NodeID   string `json:"nodeId"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
}

type WireDerpNodeRecord struct {
	RegionID          string              `json:"regionId"`
	NodeID            string              `json:"nodeId"`
	Name              string              `json:"name,omitempty"`
	Host              string              `json:"host"`
	Port              int                 `json:"port"`
	Enabled           bool                `json:"enabled"`
	Healthy           bool                `json:"healthy"`
	Stale             bool                `json:"stale"`
	Priority          int                 `json:"priority"`
	UpdatedAtMs       int64               `json:"updatedAtMs"`
	TicketKeyRotation WireTicketKeyStatus `json:"ticketKeyRotation,omitempty"`
}

type WireRelayNodeRecord struct {
	RegionID          string              `json:"regionId"`
	NodeID            string              `json:"nodeId"`
	Host              string              `json:"host"`
	UDPPort           int                 `json:"udpPort"`
	AdminPort         int                 `json:"adminPort,omitempty"`
	Enabled           bool                `json:"enabled"`
	Healthy           bool                `json:"healthy"`
	Stale             bool                `json:"stale"`
	Priority          int                 `json:"priority"`
	UpdatedAtMs       int64               `json:"updatedAtMs"`
	TicketKeyRotation WireTicketKeyStatus `json:"ticketKeyRotation,omitempty"`
}

type UpsertWireDerpNodeRequest struct {
	RegionID          string              `json:"regionId"`
	NodeID            string              `json:"nodeId"`
	Name              string              `json:"name,omitempty"`
	Host              string              `json:"host"`
	Port              int                 `json:"port,omitempty"`
	Enabled           *bool               `json:"enabled,omitempty"`
	Healthy           *bool               `json:"healthy,omitempty"`
	Priority          int                 `json:"priority,omitempty"`
	TicketKeyRotation WireTicketKeyStatus `json:"ticketKeyRotation,omitempty"`
}

type UpsertWireRelayNodeRequest struct {
	RegionID          string              `json:"regionId"`
	NodeID            string              `json:"nodeId"`
	Host              string              `json:"host"`
	UDPPort           int                 `json:"udpPort"`
	AdminPort         int                 `json:"adminPort,omitempty"`
	Enabled           *bool               `json:"enabled,omitempty"`
	Healthy           *bool               `json:"healthy,omitempty"`
	Priority          int                 `json:"priority,omitempty"`
	TicketKeyRotation WireTicketKeyStatus `json:"ticketKeyRotation,omitempty"`
}

type WireNodeHeartbeatRequest struct {
	Healthy           bool                `json:"healthy"`
	TicketKeyRotation WireTicketKeyStatus `json:"ticketKeyRotation,omitempty"`
}

type UpdateWireNodeStatusRequest struct {
	Enabled *bool `json:"enabled,omitempty"`
	Healthy *bool `json:"healthy,omitempty"`
}

type WireNodesOpsView struct {
	DerpNodes             []WireDerpNodeRecord  `json:"derpNodes"`
	RelayNodes            []WireRelayNodeRecord `json:"relayNodes"`
	RecentEvents          []WireNodeEventRecord `json:"recentEvents"`
	DerpMap               WireDerpMapView       `json:"derpMap"`
	SchedulableDerpCount  int                   `json:"schedulableDerpCount"`
	SchedulableRelayCount int                   `json:"schedulableRelayCount"`
	StaleCount            int                   `json:"staleCount"`
	TicketKeyRotation     WireTicketKeyStatus   `json:"ticketKeyRotation"`
	TicketKeyHealth       WireTicketKeyHealth   `json:"ticketKeyHealth"`
}

type WireTicketKeyStatus struct {
	Source             string `json:"source,omitempty"`
	KeyRingID          string `json:"keyRingId,omitempty"`
	SigningConfigured  bool   `json:"signingConfigured"`
	KeyRingConfigured  bool   `json:"keyRingConfigured"`
	KeyRingSize        int    `json:"keyRingSize,omitempty"`
	EffectiveKeyCount  int    `json:"effectiveKeyCount,omitempty"`
	RotationReady      bool   `json:"rotationReady"`
	AcceptsDevFallback bool   `json:"acceptsDevFallback,omitempty"`
	Available          bool   `json:"available,omitempty"`
	Error              string `json:"error,omitempty"`
	ObservedAtMs       int64  `json:"observedAtMs,omitempty"`
}

type WireTicketKeyHealth struct {
	BaselineKeyRingID string                  `json:"baselineKeyRingId,omitempty"`
	RotationReady     bool                    `json:"rotationReady"`
	Drifted           bool                    `json:"drifted"`
	UnavailableCount  int                     `json:"unavailableCount"`
	Instances         []WireTicketKeyInstance `json:"instances"`
}

type WireTicketKeyInstance struct {
	Kind     string              `json:"kind"`
	RegionID string              `json:"regionId,omitempty"`
	NodeID   string              `json:"nodeId,omitempty"`
	URL      string              `json:"url,omitempty"`
	Status   WireTicketKeyStatus `json:"status"`
	Drifted  bool                `json:"drifted"`
}

type WireNodeEventRecord struct {
	EventID     uint64 `json:"eventId"`
	NodeKind    string `json:"nodeKind"`
	RegionID    string `json:"regionId"`
	NodeID      string `json:"nodeId"`
	EventType   string `json:"eventType"`
	FromEnabled *bool  `json:"fromEnabled,omitempty"`
	ToEnabled   *bool  `json:"toEnabled,omitempty"`
	FromHealthy *bool  `json:"fromHealthy,omitempty"`
	ToHealthy   *bool  `json:"toHealthy,omitempty"`
	Reason      string `json:"reason,omitempty"`
	CreatedAtMs int64  `json:"createdAtMs"`
}

type WireNodeEventQuery struct {
	NodeKind      string
	RegionID      string
	NodeID        string
	EventType     string
	CreatedFromMs int64
	CreatedToMs   int64
	Page          int
	PageSize      int
}

type WireNodeEventListResponse struct {
	Items    []WireNodeEventRecord `json:"items"`
	Page     int                   `json:"page"`
	PageSize int                   `json:"pageSize"`
	Total    int64                 `json:"total"`
}
