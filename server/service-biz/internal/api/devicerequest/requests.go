package devicerequest

import servicepkg "github.com/slan/service-biz/internal/service"

type RegisterDevice struct {
	UserID        string `json:"userId"`
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

func (r RegisterDevice) ToInput() servicepkg.RegisterDeviceInput {
	ownerID := firstNonEmpty(r.OwnerID, r.UserID)
	name := firstNonEmpty(r.Alias, r.Name)
	return servicepkg.RegisterDeviceInput{
		OwnerID:       ownerID,
		ActorUserID:   r.ActorUserID,
		DeviceID:      r.DeviceID,
		Name:          name,
		Platform:      r.Platform,
		Alias:         r.Alias,
		OSName:        r.OSName,
		OSVersion:     r.OSVersion,
		PublicKey:     r.PublicKey,
		DeviceVersion: r.DeviceVersion,
		CountryCode:   r.CountryCode,
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
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
