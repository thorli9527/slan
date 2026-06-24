package service

type CreateDeviceGroupInput struct {
	UserID      string `json:"userId"`
	ActorUserID string `json:"actorUserId"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateDeviceGroupInput struct {
	GroupID     string `json:"groupId"`
	ActorUserID string `json:"actorUserId"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type DeleteDeviceGroupInput struct {
	GroupID     string `json:"groupId"`
	ActorUserID string `json:"actorUserId"`
}

type SetDeviceGroupsInput struct {
	UserID      string   `json:"userId"`
	ActorUserID string   `json:"actorUserId"`
	DeviceID    string   `json:"deviceId"`
	GroupIDs    []string `json:"groupIds"`
}
