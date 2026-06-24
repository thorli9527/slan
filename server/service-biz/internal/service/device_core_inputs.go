package service

type RegisterDeviceInput struct {
	OwnerID       string `json:"ownerId"`
	ActorUserID   string `json:"actorUserId"`
	DeviceID      string `json:"deviceId"`
	Name          string `json:"name"`
	Platform      string `json:"platform"`
	Alias         string `json:"alias"`
	OSName        string `json:"osName"`
	OSVersion     string `json:"osVersion"`
	PublicKey     string `json:"publicKey"`
	DeviceVersion string `json:"deviceVersion"`
	CountryCode   string `json:"countryCode"`
}

type UpdateDeviceAliasInput struct {
	DeviceID    string `json:"deviceId"`
	ActorUserID string `json:"actorUserId"`
	Alias       string `json:"alias"`
}

type UpdateDeviceRuntimeInput struct {
	DeviceID        string `json:"deviceId"`
	NetworkID       string `json:"networkId"`
	Platform        string `json:"platform"`
	RXBytesTotal    int64  `json:"rxBytesTotal"`
	TXBytesTotal    int64  `json:"txBytesTotal"`
	LastSeenAt      int64  `json:"lastSeenAt"`
	ReportedAtMS    int64  `json:"reportedAtMs"`
	Status          string `json:"status"`
	DeviceVersion   string `json:"deviceVersion"`
	NATType         string `json:"natType"`
	ActivePath      string `json:"activePath"`
	PathObservedAt  int64  `json:"pathObservedAt"`
	RelayTransport  string `json:"relayTransport"`
	RelayEndpoint   string `json:"relayEndpoint"`
	DerpNodeID      string `json:"derpNodeId"`
	PeerNodeID      string `json:"peerNodeId"`
	PathScore       int64  `json:"pathScore"`
	ObservedRttMs   int64  `json:"observedRttMs"`
	PacketLossPpm   int64  `json:"packetLossPpm"`
	RelayMtu        int    `json:"relayMtu"`
	MaxFramePayload int    `json:"maxFramePayload"`
	TicketExpiresAt string `json:"ticketExpiresAt"`
	TicketRenewDue  *bool  `json:"ticketRenewDue"`
	PathDowngrades  int64  `json:"pathDowngrades"`
	PathUpgrades    int64  `json:"pathUpgrades"`
	LastPathChange  string `json:"lastPathChange"`
}

type DeleteDeviceInput struct {
	DeviceID    string `json:"deviceId"`
	ActorUserID string `json:"actorUserId"`
}
