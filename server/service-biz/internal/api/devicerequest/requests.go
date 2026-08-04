package devicerequest

import servicepkg "github.com/slan/service-biz/internal/service"

type UpdateDeviceRuntime struct {
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

func (r UpdateDeviceRuntime) ToInput() servicepkg.UpdateDeviceRuntimeInput {
	return servicepkg.UpdateDeviceRuntimeInput{
		DeviceID:        r.DeviceID,
		NetworkID:       r.NetworkID,
		Platform:        r.Platform,
		RXBytesTotal:    r.RXBytesTotal,
		TXBytesTotal:    r.TXBytesTotal,
		LastSeenAt:      r.LastSeenAt,
		ReportedAtMS:    r.ReportedAtMS,
		Status:          r.Status,
		DeviceVersion:   r.DeviceVersion,
		NATType:         r.NATType,
		ActivePath:      r.ActivePath,
		PathObservedAt:  r.PathObservedAt,
		RelayTransport:  r.RelayTransport,
		RelayEndpoint:   r.RelayEndpoint,
		DerpNodeID:      r.DerpNodeID,
		PeerNodeID:      r.PeerNodeID,
		PathScore:       r.PathScore,
		ObservedRttMs:   r.ObservedRttMs,
		PacketLossPpm:   r.PacketLossPpm,
		RelayMtu:        r.RelayMtu,
		MaxFramePayload: r.MaxFramePayload,
		TicketExpiresAt: r.TicketExpiresAt,
		TicketRenewDue:  r.TicketRenewDue,
		PathDowngrades:  r.PathDowngrades,
		PathUpgrades:    r.PathUpgrades,
		LastPathChange:  r.LastPathChange,
	}
}
