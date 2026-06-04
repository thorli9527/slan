package biz

// GlobalIPAddress 是全局 10.0.0.0/8 地址池中的单个地址分配记录。
type GlobalIPAddress struct {
	AddressID  string `json:"addressId"`
	SubnetID   string `json:"subnetId"`
	IP         string `json:"ip"`
	CIDRBlock  string `json:"cidrBlock"`
	Offset     uint32 `json:"offset"`
	DeviceID   string `json:"deviceId,omitempty"`
	Status     string `json:"status"`
	CreatedAt  int64  `json:"createdAt"`
	AssignedAt int64  `json:"assignedAt,omitempty"`
	ReleasedAt int64  `json:"releasedAt,omitempty"`
}

// IPAMSubnet 是服务端从全局地址池切分出的地址段。
type IPAMSubnet struct {
	SubnetID          string `json:"subnetId"`
	CIDRBlock         string `json:"cidrBlock"`
	BaseIP            string `json:"baseIp"`
	PrefixLength      int    `json:"prefixLength"`
	StartOffset       uint32 `json:"startOffset"`
	EndOffset         uint32 `json:"endOffset"`
	GeneratedCapacity int    `json:"generatedCapacity"`
	Status            string `json:"status"`
	CreatedAt         int64  `json:"createdAt"`
}
