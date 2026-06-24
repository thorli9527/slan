package model

type DeviceInvite struct {
	InviteID      string `json:"inviteId"`
	InviteCode    string `json:"inviteCode,omitempty"`
	InviterUserID string `json:"inviterUserId,omitempty"`
	NetworkID     string `json:"networkId"`
	DeviceID      string `json:"deviceId"`
	UserID        string `json:"userId"`
	Status        string `json:"status"`
	CreatedAt     int64  `json:"createdAt"`
	ExpiresAt     int64  `json:"expiresAt"`
	AcceptedAt    int64  `json:"acceptedAt"`
}
