package service

type CreateSecurityGroupInput struct {
	NetworkID   string `json:"networkId"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateSecurityGroupInput struct {
	SecurityGroupID string `json:"securityGroupId"`
	Name            string `json:"name"`
	Description     string `json:"description"`
}

type CreateSecurityRuleInput struct {
	SecurityGroupID string `json:"securityGroupId"`
	Direction       string `json:"direction"`
	Protocol        string `json:"protocol"`
	PortRange       string `json:"portRange"`
	PeerType        string `json:"peerType"`
	PeerValue       string `json:"peerValue"`
	Action          string `json:"action"`
	Priority        int    `json:"priority"`
	Description     string `json:"description"`
	Enabled         bool   `json:"enabled"`
}

type UpdateSecurityRuleInput struct {
	RuleID      string `json:"ruleId"`
	Direction   string `json:"direction"`
	Protocol    string `json:"protocol"`
	PortRange   string `json:"portRange"`
	PeerType    string `json:"peerType"`
	PeerValue   string `json:"peerValue"`
	Action      string `json:"action"`
	Priority    *int   `json:"priority,omitempty"`
	Description string `json:"description"`
	Enabled     *bool  `json:"enabled,omitempty"`
}

type DeleteSecurityGroupInput struct {
	SecurityGroupID string `json:"securityGroupId"`
}

type DeleteSecurityRuleInput struct {
	RuleID string `json:"ruleId"`
}
