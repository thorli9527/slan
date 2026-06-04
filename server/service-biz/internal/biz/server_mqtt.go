package biz

import "time"

type networkChangePayload struct {
	NetworkID       string `json:"networkId"`
	ConfigVersion   int64  `json:"configVersion"`
	Reason          string `json:"reason"`
	ChangedAt       int64  `json:"changedAt"`
	ResourceType    string `json:"resourceType,omitempty"`
	Action          string `json:"action,omitempty"`
	ResourceID      string `json:"resourceId,omitempty"`
	DeviceID        string `json:"deviceId,omitempty"`
	EffectiveState  string `json:"effectiveState,omitempty"`
	VirtualIP       string `json:"virtualIp,omitempty"`
	PrefixLen       int    `json:"prefixLen,omitempty"`
	GlobalCIDR      string `json:"globalCidr,omitempty"`
	SubnetID        string `json:"subnetId,omitempty"`
	SubnetCIDR      string `json:"subnetCidr,omitempty"`
	SubnetPrefixLen int    `json:"subnetPrefixLen,omitempty"`
}

func timeNow() time.Time {
	return time.Now()
}
