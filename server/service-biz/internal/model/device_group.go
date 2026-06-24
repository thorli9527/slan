package model

type DeviceGroup struct {
	GroupID     string `json:"groupId"`
	UserID      string `json:"userId"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	CreatedAt   int64  `json:"createdAt"`
	UpdatedAt   int64  `json:"updatedAt"`
}

type DeviceGroupAssignment struct {
	UserID    string   `json:"userId"`
	DeviceID  string   `json:"deviceId"`
	GroupIDs  []string `json:"groupIds"`
	UpdatedAt int64    `json:"updatedAt"`
}
