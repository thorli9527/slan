package model

type Network struct {
	NetworkID        string `json:"networkId"`
	OwnerID          string `json:"ownerId"`
	Name             string `json:"name"`
	CIDR             string `json:"cidr,omitempty"`
	Code             string `json:"code"`
	TemplateKey      string `json:"templateKey"`
	IntraGroupPolicy string `json:"intraGroupPolicy"`
	Default          bool   `json:"default"`
	Status           string `json:"status"`
	CreatedAt        int64  `json:"createdAt"`
	UpdatedAt        int64  `json:"updatedAt"`
}
