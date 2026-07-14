package service

type SecurityGroupView struct {
	SecurityGroupID string `json:"securityGroupId"`
	NetworkID       string `json:"networkId"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	CreatedAt       int64  `json:"createdAt"`
	UpdatedAt       int64  `json:"updatedAt"`
}

type SecurityRuleView struct {
	RuleID          string `json:"ruleId"`
	SecurityGroupID string `json:"securityGroupId"`
	Direction       string `json:"direction"`
	Protocol        string `json:"protocol"`
	PortRange       string `json:"portRange"`
	CIDR            string `json:"cidr"`
	Action          string `json:"action"`
	Priority        int    `json:"priority"`
	CreatedAt       int64  `json:"createdAt"`
	UpdatedAt       int64  `json:"updatedAt"`
	PortFrom        int    `json:"portFrom"`
	PortTo          int    `json:"portTo"`
	PeerType        string `json:"peerType"`
	PeerValue       string `json:"peerValue"`
	Description     string `json:"description"`
	Enabled         bool   `json:"enabled"`
}
