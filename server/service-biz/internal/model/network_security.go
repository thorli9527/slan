package model

type SecurityGroup struct {
	SecurityGroupID string `json:"securityGroupId"`
	NetworkID       string `json:"networkId"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	CreatedAt       int64  `json:"createdAt"`
	UpdatedAt       int64  `json:"updatedAt"`
}

type SecurityRule struct {
	RuleID          string `json:"ruleId"`
	SecurityGroupID string `json:"securityGroupId"`
	Direction       string `json:"direction"`
	Protocol        string `json:"protocol"`
	PortRange       string `json:"portRange"`
	CIDR            string `json:"cidr"`
	PeerType        string `json:"peerType"`
	PeerValue       string `json:"peerValue"`
	Action          string `json:"action"`
	Priority        int    `json:"priority"`
	Description     string `json:"description,omitempty"`
	Enabled         bool   `json:"enabled"`
	CreatedAt       int64  `json:"createdAt"`
	UpdatedAt       int64  `json:"updatedAt"`
}
