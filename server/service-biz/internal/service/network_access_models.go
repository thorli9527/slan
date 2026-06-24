package service

type SecurityGroupView struct {
	SecurityGroupID string `json:"securityGroupId"`
	NetworkID       string `json:"networkId"`
	Name            string `json:"name"`
	Description     string `json:"description"`
	CreatedAt       int64  `json:"createdAt"`
	UpdatedAt       int64  `json:"updatedAt"`
}

type PublicMappingView struct {
	MappingID    string `json:"mappingId"`
	NetworkID    string `json:"networkId"`
	Name         string `json:"name"`
	PublicDomain string `json:"publicDomain"`
	SourceRecord string `json:"sourceRecord"`
	Protocol     string `json:"protocol"`
	InternalIP   string `json:"internalIp"`
	InternalPort int    `json:"internalPort"`
	ExternalPort int    `json:"externalPort"`
	AccessMode   string `json:"accessMode"`
	TLSMode      string `json:"tlsMode"`
	TargetType   string `json:"targetType"`
	Status       string `json:"status"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
	DeviceID     string `json:"deviceId"`
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
