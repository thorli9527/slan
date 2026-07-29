package service

import "context"

type NetworkEventType string

const (
	NetworkEventVersion            NetworkEventType = "network_version"
	NetworkEventSnapshot           NetworkEventType = "network_snapshot"
	NetworkEventMemberAdded        NetworkEventType = "member_added"
	NetworkEventMemberRemoved      NetworkEventType = "member_removed"
	NetworkEventMemberUpdated      NetworkEventType = "member_updated"
	NetworkEventMemberOnline       NetworkEventType = "member_online"
	NetworkEventMemberOffline      NetworkEventType = "member_offline"
	NetworkEventDeviceGroupAdded   NetworkEventType = "device_group_added"
	NetworkEventDeviceGroupRemoved NetworkEventType = "device_group_removed"
	NetworkEventDeviceGroupUpdated NetworkEventType = "device_group_updated"
	NetworkEventACLChanged         NetworkEventType = "acl_changed"
	NetworkEventDNSChanged         NetworkEventType = "resolver_changed"
	NetworkEventConfigChanged      NetworkEventType = "network_config_changed"
	NetworkEventPeerPathChanged    NetworkEventType = "peer_path_changed"
)

type NetworkEventEnvelope struct {
	Type       string           `json:"type"`
	NetworkID  string           `json:"networkId"`
	Version    uint64           `json:"version"`
	EventID    string           `json:"eventId"`
	EventType  NetworkEventType `json:"eventType"`
	OccurredAt int64            `json:"occurredAt"`
	Payload    any              `json:"payload"`
}

type NetworkEventNetworkView struct {
	NetworkID        string   `json:"networkId"`
	Name             string   `json:"name"`
	Tags             []string `json:"tags"`
	DefaultACLPolicy string   `json:"defaultAclPolicy"`
	UpdatedAt        int64    `json:"updatedAt"`
}

type NetworkEventMemberView struct {
	DeviceID      string   `json:"deviceId"`
	DeviceName    string   `json:"deviceName"`
	VirtualIP     string   `json:"virtualIp"`
	Online        bool     `json:"online"`
	LastSeenAt    int64    `json:"lastSeenAt"`
	Tags          []string `json:"tags"`
	GroupIDs      []string `json:"groupIds"`
	DeviceVersion string   `json:"deviceVersion"`
	Platform      string   `json:"platform"`
}

type NetworkEventDeviceGroupView struct {
	GroupID         string   `json:"groupId"`
	Name            string   `json:"name"`
	Tags            []string `json:"tags"`
	MemberDeviceIDs []string `json:"memberDeviceIds"`
	UpdatedAt       int64    `json:"updatedAt"`
}

type NetworkEventDNSZoneView struct {
	ZoneID    string `json:"zoneId"`
	NetworkID string `json:"networkId"`
	ZoneName  string `json:"zoneName"`
	UpdatedAt int64  `json:"updatedAt"`
}

type NetworkEventDNSRecordView struct {
	RecordID       string `json:"recordId"`
	ZoneID         string `json:"zoneId"`
	NetworkID      string `json:"networkId"`
	Name           string `json:"name"`
	FQDN           string `json:"fqdn"`
	RecordType     string `json:"recordType"`
	Value          string `json:"value"`
	TargetDeviceID string `json:"targetDeviceId"`
	TargetIP       string `json:"targetIp"`
	CNAME          string `json:"cname"`
	Port           int    `json:"port"`
	TTL            int    `json:"ttl"`
	Enabled        bool   `json:"enabled"`
	UpdatedAt      int64  `json:"updatedAt"`
}

type NetworkEventDNSConfigView struct {
	Servers                   []string `json:"servers"`
	SearchDomains             []string `json:"searchDomains"`
	SplitDomains              []string `json:"splitDomains"`
	FallbackToSystemResolvers bool     `json:"fallbackToSystemResolvers"`
}

type NetworkEventACLRuleView struct {
	RuleID          string   `json:"ruleId"`
	Priority        int      `json:"priority"`
	Action          string   `json:"action"`
	Direction       string   `json:"direction"`
	Protocol        string   `json:"protocol"`
	PortRanges      []string `json:"portRanges"`
	SourceType      string   `json:"sourceType"`
	SourceValues    []string `json:"sourceValues"`
	SourceDeviceIDs []string `json:"sourceDeviceIds"`
	SourceGroupIDs  []string `json:"sourceGroupIds"`
	TargetType      string   `json:"targetType"`
	TargetValues    []string `json:"targetValues"`
	TargetDeviceIDs []string `json:"targetDeviceIds"`
	TargetGroupIDs  []string `json:"targetGroupIds"`
	Enabled         bool     `json:"enabled"`
	UpdatedAt       int64    `json:"updatedAt"`
}

type NetworkEventPeerPathView struct {
	PeerDeviceID  string `json:"peerDeviceId"`
	PathType      string `json:"pathType"`
	RelayRegionID string `json:"relayRegionId"`
	Reachable     bool   `json:"reachable"`
	UpdatedAt     int64  `json:"updatedAt"`
}

type NetworkSnapshotPayload struct {
	Network      NetworkEventNetworkView       `json:"network"`
	Members      []NetworkEventMemberView      `json:"members"`
	DeviceGroups []NetworkEventDeviceGroupView `json:"deviceGroups"`
	DNSConfig    NetworkEventDNSConfigView     `json:"resolverConfig"`
	DNSZones     []NetworkEventDNSZoneView     `json:"resolverZones"`
	DNSRecords   []NetworkEventDNSRecordView   `json:"resolverRecords"`
	ACLRules     []NetworkEventACLRuleView     `json:"aclRules"`
	PeerPaths    []NetworkEventPeerPathView    `json:"peerPaths"`
}

type NetworkSnapshotResponse struct {
	NetworkID string                 `json:"networkId"`
	Version   uint64                 `json:"version"`
	Snapshot  NetworkSnapshotPayload `json:"snapshot"`
}

type NetworkEventMemberPayload struct {
	Member NetworkEventMemberView `json:"member"`
}

type NetworkEventMemberRemovedPayload struct {
	DeviceID string `json:"deviceId"`
}

type NetworkEventPresencePayload struct {
	DeviceID   string `json:"deviceId"`
	Online     bool   `json:"online"`
	LastSeenAt int64  `json:"lastSeenAt"`
}

type NetworkEventDeviceGroupPayload struct {
	Group NetworkEventDeviceGroupView `json:"group"`
}

type NetworkEventDeviceGroupRemovedPayload struct {
	GroupID string `json:"groupId"`
}

type NetworkEventACLChangedPayload struct {
	Rules []NetworkEventACLRuleView `json:"rules"`
}

type NetworkEventDNSChangedPayload struct {
	Config  NetworkEventDNSConfigView   `json:"config"`
	Zones   []NetworkEventDNSZoneView   `json:"zones"`
	Records []NetworkEventDNSRecordView `json:"records"`
}

type NetworkEventConfigChangedPayload struct {
	Network NetworkEventNetworkView `json:"network"`
}

type NetworkEventPeerPathChangedPayload struct {
	Paths []NetworkEventPeerPathView `json:"paths"`
}

type NetworkSnapshotUseCase interface {
	NetworkSnapshot(ctx context.Context, networkID, deviceID string) (NetworkSnapshotResponse, error)
}
