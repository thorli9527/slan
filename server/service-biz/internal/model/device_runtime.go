package model

type DeviceEndpoint struct {
	Type      string `json:"type"`
	Address   string `json:"address"`
	UpdatedAt int64  `json:"updatedAt,omitempty"`
}

// DeviceRuntimeState is ephemeral client-reported state. It must never be
// persisted as device configuration in PostgreSQL.
type DeviceRuntimeState struct {
	DeviceID           string `json:"deviceId"`
	ApplicationState   string `json:"applicationState"`
	Activated          bool   `json:"activated"`
	NetworkEnabled     bool   `json:"networkEnabled"`
	VirtualIP          string `json:"virtualIp,omitempty"`
	LastSeenAt         int64  `json:"lastSeenAt"`
	LastHeartbeatAt    int64  `json:"lastHeartbeatAt,omitempty"`
	LastRuntimeStateAt int64  `json:"lastRuntimeStateAt,omitempty"`
}
