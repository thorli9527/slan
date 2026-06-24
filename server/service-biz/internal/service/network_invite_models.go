package service

type DeviceInviteView struct {
	InviteID         string `json:"inviteId"`
	InviteCode       string `json:"inviteCode"`
	InviterUserID    string `json:"inviterUserId"`
	NetworkID        string `json:"networkId"`
	DeviceID         string `json:"deviceId"`
	UserID           string `json:"userId"`
	Status           string `json:"status"`
	CreatedAt        int64  `json:"createdAt"`
	ExpiresAt        int64  `json:"expiresAt"`
	InviterEmail     string `json:"inviterEmail"`
	AcceptedDeviceID string `json:"acceptedDeviceId"`
	AcceptedUserID   string `json:"acceptedUserId"`
	AcceptedAt       int64  `json:"acceptedAt"`
}

type NetworkDeviceView struct {
	NetworkID   string `json:"networkId"`
	DeviceID    string `json:"deviceId"`
	Enabled     bool   `json:"enabled"`
	Status      string `json:"status"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
	OwnerUserID string `json:"ownerUserId"`
	Alias       string `json:"alias"`
}
