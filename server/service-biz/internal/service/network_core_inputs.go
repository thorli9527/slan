package service

type CreateNetworkInput struct {
	OwnerID          string `json:"ownerId"`
	ActorUserID      string `json:"actorUserId"`
	Name             string `json:"name"`
	CIDR             string `json:"cidr"`
	IntraGroupPolicy string `json:"intraGroupPolicy"`
	Default          bool   `json:"default"`
}

type UpdateNetworkInput struct {
	NetworkID        string `json:"networkId"`
	ActorUserID      string `json:"actorUserId"`
	Name             string `json:"name"`
	CIDR             string `json:"cidr"`
	IntraGroupPolicy string `json:"intraGroupPolicy"`
	Default          *bool  `json:"default,omitempty"`
	Status           string `json:"status"`
}

type DeleteNetworkInput struct {
	NetworkID   string `json:"networkId"`
	ActorUserID string `json:"actorUserId"`
}
