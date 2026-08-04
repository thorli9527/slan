package model

type Network struct {
	NetworkID        string `json:"networkId"`
	Name             string `json:"name"`
	CIDR             string `json:"cidr,omitempty"`
	IntraGroupPolicy string `json:"intraGroupPolicy"`
	Default          bool   `json:"default"`
	Status           string `json:"status"`
	CreatedAt        int64  `json:"createdAt"`
	UpdatedAt        int64  `json:"updatedAt"`
}

type NetworkConfigVersion struct {
	NetworkID string `json:"networkId"`
	Version   int64  `json:"version"`
	Reason    string `json:"reason"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}
