package service

type CreateDeviceGroupInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateDeviceGroupInput struct {
	GroupID     string `json:"groupId"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type DeleteDeviceGroupInput struct {
	GroupID string `json:"groupId"`
}

type SetDeviceGroupsInput struct {
	DeviceID string   `json:"deviceId"`
	GroupIDs []string `json:"groupIds"`
}

type AddNetworkDeviceGroupInput struct {
	NetworkID string `json:"networkId"`
	GroupID   string `json:"groupId"`
}

type RemoveNetworkDeviceGroupInput struct {
	NetworkID string `json:"networkId"`
	GroupID   string `json:"groupId"`
}
