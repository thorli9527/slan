package model

type NetworkMemberStatus string

const (
	NetworkMemberStatusActive   NetworkMemberStatus = "active"
	NetworkMemberStatusDisabled NetworkMemberStatus = "disabled"
	NetworkMemberStatusRemoved  NetworkMemberStatus = "removed"
)

type DevicePresenceStatus string

const (
	DevicePresenceStatusOffline   DevicePresenceStatus = "offline"
	DevicePresenceStatusConnected DevicePresenceStatus = "connected"
	DevicePresenceStatusActive    DevicePresenceStatus = "active"
	DevicePresenceStatusStale     DevicePresenceStatus = "stale"
)

type NetworkDevice struct {
	NetworkID          string               `json:"networkId"`
	DeviceID           string               `json:"deviceId"`
	Enabled            bool                 `json:"enabled"`
	MemberStatus       NetworkMemberStatus  `json:"memberStatus"`
	PresenceStatus     DevicePresenceStatus `json:"presenceStatus"`
	MQTTConnected      bool                 `json:"mqttConnected"`
	VirtualIP          string               `json:"virtualIp,omitempty"`
	LastSeenAt         int64                `json:"lastSeenAt,omitempty"`
	LastHeartbeatAt    int64                `json:"lastHeartbeatAt,omitempty"`
	LastRuntimeStateAt int64                `json:"lastRuntimeStateAt,omitempty"`
	LastEndpointAt     int64                `json:"lastEndpointAt,omitempty"`
	LastPathHealthAt   int64                `json:"lastPathHealthAt,omitempty"`
	Endpoints          []DeviceEndpoint     `json:"endpoints,omitempty"`
	NATType            string               `json:"natType,omitempty"`
	ActivePath         string               `json:"activePath,omitempty"`
	PathObservedAt     int64                `json:"pathObservedAt,omitempty"`
	RelayTransport     string               `json:"relayTransport,omitempty"`
	RelayEndpoint      string               `json:"relayEndpoint,omitempty"`
	DerpNodeID         string               `json:"derpNodeId,omitempty"`
	PeerNodeID         string               `json:"peerNodeId,omitempty"`
	PathScore          int64                `json:"pathScore,omitempty"`
	ObservedRttMs      int64                `json:"observedRttMs,omitempty"`
	PacketLossPpm      int64                `json:"packetLossPpm,omitempty"`
	RelayMtu           int                  `json:"relayMtu,omitempty"`
	MaxFramePayload    int                  `json:"maxFramePayload,omitempty"`
	TicketExpiresAt    string               `json:"ticketExpiresAt,omitempty"`
	TicketRenewDue     bool                 `json:"ticketRenewDue,omitempty"`
	PathDowngrades     int64                `json:"pathDowngrades,omitempty"`
	PathUpgrades       int64                `json:"pathUpgrades,omitempty"`
	LastPathChange     string               `json:"lastPathChange,omitempty"`
	CreatedAt          int64                `json:"createdAt"`
	UpdatedAt          int64                `json:"updatedAt"`
}
