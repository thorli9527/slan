package service

type CreatePublicMappingInput struct {
	NetworkID    string `json:"networkId"`
	ActorUserID  string `json:"actorUserId"`
	Name         string `json:"name"`
	PublicDomain string `json:"publicDomain"`
	SourceRecord string `json:"sourceRecord"`
	DeviceID     string `json:"deviceId"`
	Protocol     string `json:"protocol"`
	InternalIP   string `json:"internalIp"`
	InternalPort int    `json:"internalPort"`
	ExternalPort int    `json:"externalPort"`
	AccessMode   string `json:"accessMode"`
	TLSMode      string `json:"tlsMode"`
}

type UpdatePublicMappingInput struct {
	MappingID    string `json:"mappingId"`
	ActorUserID  string `json:"actorUserId"`
	Name         string `json:"name"`
	PublicDomain string `json:"publicDomain"`
	SourceRecord string `json:"sourceRecord"`
	DeviceID     string `json:"deviceId"`
	Protocol     string `json:"protocol"`
	InternalIP   string `json:"internalIp"`
	InternalPort int    `json:"internalPort"`
	ExternalPort int    `json:"externalPort"`
	AccessMode   string `json:"accessMode"`
	TLSMode      string `json:"tlsMode"`
	Status       string `json:"status"`
}

type CreateSecurityGroupInput struct {
	NetworkID   string `json:"networkId"`
	ActorUserID string `json:"actorUserId"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UpdateSecurityGroupInput struct {
	SecurityGroupID string `json:"securityGroupId"`
	ActorUserID     string `json:"actorUserId"`
	Name            string `json:"name"`
	Description     string `json:"description"`
}

type CreateSecurityRuleInput struct {
	SecurityGroupID string `json:"securityGroupId"`
	ActorUserID     string `json:"actorUserId"`
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
	ActorUserID string `json:"actorUserId"`
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

type DeletePublicMappingInput struct {
	MappingID   string `json:"mappingId"`
	ActorUserID string `json:"actorUserId"`
}

type DeleteSecurityGroupInput struct {
	SecurityGroupID string `json:"securityGroupId"`
	ActorUserID     string `json:"actorUserId"`
}

type DeleteSecurityRuleInput struct {
	RuleID      string `json:"ruleId"`
	ActorUserID string `json:"actorUserId"`
}
