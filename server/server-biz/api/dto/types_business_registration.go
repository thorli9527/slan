package dto

type RegisterDeviceRequest struct {
	DeviceID      string `json:"deviceId,omitempty"`
	Name          string `json:"name"`
	Platform      string `json:"platform"`
	DeviceVersion string `json:"deviceVersion,omitempty"`
	CountryCode   string `json:"countryCode,omitempty"`
	PublicKey     string `json:"publicKey"`
}

type DeviceNetworkStateRequest struct {
	DeviceID         string `json:"deviceId,omitempty"`
	NetworkID        string `json:"networkId,omitempty"`
	ControlReachable bool   `json:"controlReachable"`
	NetworkOnline    bool   `json:"networkOnline"`
	TunnelUp         bool   `json:"tunnelUp"`
	LastProbeOK      bool   `json:"lastProbeOk"`
	VirtualIP        string `json:"virtualIp,omitempty"`
	ReportedAt       int64  `json:"reportedAt,omitempty"`
}

type DeviceNetworkState struct {
	DeviceID         string `json:"deviceId"`
	NetworkID        string `json:"networkId"`
	ControlReachable bool   `json:"controlReachable"`
	NetworkOnline    bool   `json:"networkOnline"`
	TunnelUp         bool   `json:"tunnelUp"`
	LastProbeOK      bool   `json:"lastProbeOk"`
	VirtualIP        string `json:"virtualIp,omitempty"`
	LastSeenAt       int64  `json:"lastSeenAt"`
	UpdatedAt        int64  `json:"updatedAt"`
}

type Device struct {
	DeviceID             string              `json:"deviceId"`
	Name                 string              `json:"name"`
	OwnerEmail           string              `json:"ownerEmail,omitempty"`
	Platform             string              `json:"platform"`
	DeviceVersion        string              `json:"deviceVersion,omitempty"`
	CountryCode          string              `json:"countryCode,omitempty"`
	Status               string              `json:"status"`
	CurrentVirtualIP     string              `json:"currentVirtualIp,omitempty"`
	LinkStatus           string              `json:"linkStatus,omitempty"`
	ConnectivityProtocol string              `json:"connectivityProtocol,omitempty"`
	JoinedAt             int64               `json:"joinedAt,omitempty"`
	MembershipStatus     string              `json:"membershipStatus,omitempty"`
	NetworkRole          string              `json:"networkRole,omitempty"`
	CreatedAt            int64               `json:"createdAt,omitempty"`
	PublicKey            string              `json:"publicKey,omitempty"`
	NetworkIDs           []string            `json:"networkIds,omitempty"`
	MQTT                 *MQTTCredential     `json:"mqtt,omitempty"`
	NetworkState         *DeviceNetworkState `json:"networkState,omitempty"`
}

type MQTTCredential struct {
	BrokerURL   string `json:"brokerUrl"`
	ClientID    string `json:"clientId"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	TopicPrefix string `json:"topicPrefix"`
	ExpiresAt   int64  `json:"expiresAt,omitempty"`
}

type RegisterNodeRequest struct {
	DeviceID      string   `json:"deviceId"`
	NodeID        string   `json:"nodeId"`
	NodePublicKey string   `json:"nodePublicKey"`
	Capabilities  []string `json:"capabilities,omitempty"`
}

type Node struct {
	NodeID        string   `json:"nodeId"`
	DeviceID      string   `json:"deviceId"`
	NodePublicKey string   `json:"nodePublicKey"`
	NetworkIDs    []string `json:"networkIds,omitempty"`
	Capabilities  []string `json:"capabilities,omitempty"`
}
