package model

import "time"

type PathKind string

const (
	PathLANUDP     PathKind = "lan_udp"
	PathIPv6UDP    PathKind = "ipv6_udp"
	PathDirectUDP  PathKind = "direct_udp"
	PathRelayUDP   PathKind = "relay_udp"
	PathDerpTCP443 PathKind = "derp_tcp_tls_443"
)

type PathProbe struct {
	Path             PathKind `json:"path"`
	Reachable        bool     `json:"reachable"`
	RTTMs            int      `json:"rttMs,omitempty"`
	LossPPM          int      `json:"lossPpm,omitempty"`
	JitterMs         int      `json:"jitterMs,omitempty"`
	ConsecutiveFails int      `json:"consecutiveFails,omitempty"`
	MTU              int      `json:"mtu,omitempty"`
	ObservedAt       int64    `json:"observedAt,omitempty"`
}

type RelayTicket struct {
	TicketID     string    `json:"ticketId,omitempty"`
	PeerID       string    `json:"peerId,omitempty"`
	SessionID    string    `json:"sessionId,omitempty"`
	Path         PathKind  `json:"path,omitempty"`
	RegionID     string    `json:"regionId,omitempty"`
	NodeID       string    `json:"nodeId,omitempty"`
	Host         string    `json:"host,omitempty"`
	UDPPort      int       `json:"udpPort,omitempty"`
	Present      bool      `json:"present"`
	ExpiresAt    time.Time `json:"expiresAt,omitempty"`
	ExpiresInMs  int64     `json:"expiresInMs,omitempty"`
	RenewAfterMs int64     `json:"renewAfterMs,omitempty"`
	Signature    string    `json:"signature,omitempty"`
}

type RelayNode struct {
	RegionID  string `json:"regionId"`
	NodeID    string `json:"nodeId"`
	Host      string `json:"host"`
	UDPPort   int    `json:"udpPort"`
	AdminPort int    `json:"adminPort,omitempty"`
	Enabled   bool   `json:"enabled"`
	Healthy   bool   `json:"healthy"`
	Stale     bool   `json:"stale"`
	Priority  int    `json:"priority"`
}

type DerpNode struct {
	RegionID string `json:"regionId"`
	NodeID   string `json:"nodeId"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
}

type DerpRegion struct {
	RegionID string     `json:"regionId"`
	Name     string     `json:"name"`
	Nodes    []DerpNode `json:"nodes"`
}

type DerpMap struct {
	PreferredRegionID string       `json:"preferredRegionId,omitempty"`
	Regions           []DerpRegion `json:"regions"`
}

type DerpTicket struct {
	TicketID     string    `json:"ticketId"`
	PeerID       string    `json:"peerId"`
	NetworkID    string    `json:"networkId,omitempty"`
	Path         PathKind  `json:"path"`
	RegionID     string    `json:"regionId"`
	NodeID       string    `json:"nodeId"`
	ExpiresAt    time.Time `json:"expiresAt"`
	ExpiresInMs  int64     `json:"expiresInMs,omitempty"`
	RenewAfterMs int64     `json:"renewAfterMs,omitempty"`
	Signature    string    `json:"signature,omitempty"`
}

type PeerSnapshot struct {
	PeerID                  string             `json:"peerId"`
	ActivePath              PathKind           `json:"activePath,omitempty"`
	SupportsLANDirect       bool               `json:"supportsLanDirect"`
	SupportsIPv6Direct      bool               `json:"supportsIpv6Direct"`
	SupportsDirectUDP       bool               `json:"supportsDirectUdp"`
	SupportsRelayUDP        bool               `json:"supportsRelayUdp"`
	SupportsDerpTCPTLS443   bool               `json:"supportsDerpTcpTls443"`
	EndpointChanged         bool               `json:"endpointChanged"`
	PreferIPv6              bool               `json:"preferIpv6"`
	PreferLAN               bool               `json:"preferLan"`
	KeepaliveIntervalSecs   int                `json:"keepaliveIntervalSecs,omitempty"`
	Probes                  []PathProbe        `json:"probes"`
	RelayTicket             RelayTicket        `json:"relayTicket"`
	RecentPathDowngrades    int                `json:"recentPathDowngrades,omitempty"`
	RecentPathUpgrades      int                `json:"recentPathUpgrades,omitempty"`
	RequireMtuRefresh       bool               `json:"requireMtuRefresh"`
	AllowEndpointRoaming    bool               `json:"allowEndpointRoaming"`
	AllowFastReselection    bool               `json:"allowFastReselection"`
	AllowRelayTicketRenewal bool               `json:"allowRelayTicketRenewal"`
	DerpHealth              []DerpHealthSample `json:"derpHealth,omitempty"`
}

type Endpoint struct {
	Kind       string `json:"kind"`
	Address    string `json:"address"`
	Port       int    `json:"port"`
	Reachable  bool   `json:"reachable,omitempty"`
	ObservedAt int64  `json:"observedAt,omitempty"`
}

type PeerRegistration struct {
	PeerID                  string     `json:"peerId"`
	NetworkID               string     `json:"networkId,omitempty"`
	NodeID                  string     `json:"nodeId,omitempty"`
	PublicKey               string     `json:"publicKey,omitempty"`
	VirtualIPs              []string   `json:"virtualIps,omitempty"`
	AllowedIPs              []string   `json:"allowedIps,omitempty"`
	SupportsLANDirect       bool       `json:"supportsLanDirect"`
	SupportsIPv6Direct      bool       `json:"supportsIpv6Direct"`
	SupportsDirectUDP       bool       `json:"supportsDirectUdp"`
	SupportsRelayUDP        bool       `json:"supportsRelayUdp"`
	SupportsDerpTCPTLS443   bool       `json:"supportsDerpTcpTls443"`
	PreferIPv6              bool       `json:"preferIpv6"`
	PreferLAN               bool       `json:"preferLan"`
	AllowEndpointRoaming    bool       `json:"allowEndpointRoaming"`
	AllowFastReselection    bool       `json:"allowFastReselection"`
	AllowRelayTicketRenewal bool       `json:"allowRelayTicketRenewal"`
	KeepaliveIntervalSecs   int        `json:"keepaliveIntervalSecs,omitempty"`
	Endpoints               []Endpoint `json:"endpoints,omitempty"`
}

type PeerRecord struct {
	PeerRegistration
	ActivePath           PathKind           `json:"activePath,omitempty"`
	RecentPathDowngrades int                `json:"recentPathDowngrades,omitempty"`
	RecentPathUpgrades   int                `json:"recentPathUpgrades,omitempty"`
	RequireMtuRefresh    bool               `json:"requireMtuRefresh"`
	EndpointChanged      bool               `json:"endpointChanged"`
	Probes               []PathProbe        `json:"probes,omitempty"`
	DerpHealth           []DerpHealthSample `json:"derpHealth,omitempty"`
	RelayTicket          RelayTicket        `json:"relayTicket,omitempty"`
	UpdatedAt            int64              `json:"updatedAt"`
}

type RegisterPeerRequest struct {
	Peer PeerRegistration `json:"peer"`
}

type RegisterPeerResponse struct {
	Peer PeerRecord `json:"peer"`
}

type UpdateEndpointsRequest struct {
	PeerID    string     `json:"peerId"`
	Endpoints []Endpoint `json:"endpoints"`
}

type ReportPathHealthRequest struct {
	PeerID string      `json:"peerId"`
	Probes []PathProbe `json:"probes"`
}

type DerpHealthSample struct {
	RegionID   string `json:"regionId"`
	NodeID     string `json:"nodeId"`
	Reachable  bool   `json:"reachable"`
	RTTMs      int    `json:"rttMs,omitempty"`
	ObservedAt int64  `json:"observedAt,omitempty"`
}

type ReportDerpHealthRequest struct {
	PeerID  string             `json:"peerId"`
	Samples []DerpHealthSample `json:"samples"`
}

type UpdateActivePathRequest struct {
	PeerID string   `json:"peerId"`
	Path   PathKind `json:"path"`
}

type IssueRelayTicketRequest struct {
	PeerID       string `json:"peerId"`
	TTLSeconds   int64  `json:"ttlSeconds,omitempty"`
	RenewAfterMs int64  `json:"renewAfterMs,omitempty"`
}

type IssueRelayTicketResponse struct {
	Ticket RelayTicket `json:"ticket"`
}

type GetDerpMapResponse struct {
	Map DerpMap `json:"map"`
}

type IssueDerpTicketRequest struct {
	PeerID       string `json:"peerId"`
	RegionID     string `json:"regionId"`
	NodeID       string `json:"nodeId"`
	TTLSeconds   int64  `json:"ttlSeconds,omitempty"`
	RenewAfterMs int64  `json:"renewAfterMs,omitempty"`
}

type IssueDerpTicketResponse struct {
	Ticket DerpTicket `json:"ticket"`
}

type PeerAuthzView struct {
	PeerID      string   `json:"peerId"`
	NetworkID   string   `json:"networkId,omitempty"`
	NodeID      string   `json:"nodeId,omitempty"`
	Enabled     bool     `json:"enabled"`
	VirtualIPs  []string `json:"virtualIps,omitempty"`
	AllowedIPs  []string `json:"allowedIps,omitempty"`
	QuotaPolicy string   `json:"quotaPolicy,omitempty"`
}

type PeerRuntimeConfigView struct {
	PeerID                string     `json:"peerId"`
	NetworkID             string     `json:"networkId,omitempty"`
	NodeID                string     `json:"nodeId,omitempty"`
	VirtualIPs            []string   `json:"virtualIps,omitempty"`
	AllowedIPs            []string   `json:"allowedIps,omitempty"`
	KeepaliveIntervalSecs int        `json:"keepaliveIntervalSecs,omitempty"`
	NetworkEnabled        bool       `json:"networkEnabled"`
	PreferredPath         PathKind   `json:"preferredPath,omitempty"`
	Endpoints             []Endpoint `json:"endpoints,omitempty"`
}

type NetworkTopologyView struct {
	NetworkID string       `json:"networkId"`
	Peers     []PeerRecord `json:"peers"`
}

type PathPlanRequest struct {
	PeerID string       `json:"peerId,omitempty"`
	Peer   PeerSnapshot `json:"peer"`
}

type ScoredPath struct {
	Path    PathKind `json:"path"`
	Score   int      `json:"score"`
	Reason  string   `json:"reason"`
	MTU     int      `json:"mtu,omitempty"`
	Primary bool     `json:"primary,omitempty"`
}

type KeepalivePlan struct {
	IntervalSecs int    `json:"intervalSecs"`
	Mode         string `json:"mode"`
}

type MtuPlan struct {
	ProbeRequired bool `json:"probeRequired"`
	TargetMTU     int  `json:"targetMtu,omitempty"`
}

type RelayTicketPlan struct {
	RenewRequired bool  `json:"renewRequired"`
	RenewWindowMs int64 `json:"renewWindowMs,omitempty"`
}

type RoamingPlan struct {
	Apply bool   `json:"apply"`
	Mode  string `json:"mode,omitempty"`
}

type PathPlan struct {
	PreferredPath      PathKind        `json:"preferredPath"`
	DegradedReason     string          `json:"degradedReason,omitempty"`
	FallbackOrder      []PathKind      `json:"fallbackOrder"`
	ScoredPaths        []ScoredPath    `json:"scoredPaths"`
	Keepalive          KeepalivePlan   `json:"keepalive"`
	MTU                MtuPlan         `json:"mtu"`
	Roaming            RoamingPlan     `json:"roaming"`
	RelayTicket        RelayTicketPlan `json:"relayTicket"`
	RelayCandidates    []RelayNode     `json:"relayCandidates,omitempty"`
	DerpCandidates     []DerpNode      `json:"derpCandidates,omitempty"`
	DerpTicket         *DerpTicket     `json:"derpTicket,omitempty"`
	FastReselection    bool            `json:"fastReselection"`
	IPv6Preferred      bool            `json:"ipv6Preferred"`
	LANDirectPreferred bool            `json:"lanDirectPreferred"`
}

type TicketKeyStatus struct {
	Source             string `json:"source"`
	KeyRingID          string `json:"keyRingId"`
	SigningConfigured  bool   `json:"signingConfigured"`
	KeyRingConfigured  bool   `json:"keyRingConfigured"`
	EffectiveKeyCount  int    `json:"effectiveKeyCount"`
	RotationReady      bool   `json:"rotationReady"`
	AcceptsDevFallback bool   `json:"acceptsDevFallback"`
}
