package service

type CreateDeviceInviteInput struct {
	NetworkID     string `json:"networkId"`
	DeviceID      string `json:"deviceId"`
	UserID        string `json:"userId"`
	InviterUserID string `json:"inviterUserId"`
	OwnerUserID   string `json:"ownerUserId"`
	TTLSeconds    int64  `json:"ttlSeconds"`
}

type AcceptDeviceInviteInput struct {
	InviteID    string `json:"inviteId"`
	InviteCode  string `json:"inviteCode"`
	DeviceID    string `json:"deviceId"`
	UserID      string `json:"userId"`
	ActorUserID string `json:"actorUserId"`
	Alias       string `json:"alias"`
}
