package service

type RevokeUserManagedSessionInput struct {
	UserID       string `json:"userId"`
	ActorUserID  string `json:"actorUserId"`
	SessionID    string `json:"sessionId"`
}

type RevokeDeviceManagedSessionInput struct {
	DeviceID      string `json:"deviceId"`
	ActorUserID   string `json:"actorUserId"`
	SessionID     string `json:"sessionId"`
}
