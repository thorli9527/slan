package service

type DeviceGroupMemberView struct {
	GroupID  string `json:"groupId"`
	DeviceID string `json:"deviceId"`
	AddedAt  int64  `json:"addedAt"`
}

type DeviceGroupView struct {
	GroupID     string `json:"groupId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

type DeviceGroupCollectionView struct {
	Items   []DeviceGroupView       `json:"items"`
	Members []DeviceGroupMemberView `json:"members"`
}
