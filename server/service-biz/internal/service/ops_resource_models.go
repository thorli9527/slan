package service

type OpsNetworkInput struct {
	Name             string `json:"name"`
	IntraGroupPolicy string `json:"intraGroupPolicy"`
	Status           string `json:"status"`
}

type OpsNetworkView struct {
	NetworkID        string   `json:"networkId"`
	Name             string   `json:"name"`
	IntraGroupPolicy string   `json:"intraGroupPolicy"`
	Status           string   `json:"status"`
	DeviceIDs        []string `json:"deviceIds"`
	DeviceGroupIDs   []string `json:"deviceGroupIds"`
	CreatedAt        int64    `json:"createdAt"`
	UpdatedAt        int64    `json:"updatedAt"`
}

type OpsDeviceGroupInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type OpsDeviceGroupCollectionView struct {
	Items   []DeviceGroupView       `json:"items"`
	Members []DeviceGroupMemberView `json:"members"`
}
