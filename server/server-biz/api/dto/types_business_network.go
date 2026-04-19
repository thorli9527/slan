package dto

// CreateNetworkRequest 用于创建逻辑网络。
// 注意：这里的 CIDR 指的是随网络一起创建的默认子网 CIDR。
type CreateNetworkRequest struct {
	// Name 是展示给用户的逻辑网络名称。
	Name string `json:"name"`
	// Description 是给运维或业务使用的可选说明。
	Description string `json:"description,omitempty"`
	// CIDR 是该网络创建时默认子网的 CIDR。
	CIDR string `json:"cidr"`
}

// CreateSubnetRequest 用于在已有网络中创建额外子网。
type CreateSubnetRequest struct {
	// Name 是网络内唯一的子网名称。
	Name string `json:"name"`
	// CIDR 是该子网管理的地址段。
	CIDR string `json:"cidr"`
	// GatewayIP 是预留的网关或虚拟路由地址。
	GatewayIP string `json:"gatewayIp,omitempty"`
	// AllocationStartIP 定义可分配地址范围的起始地址。
	AllocationStartIP string `json:"allocationStartIp,omitempty"`
	// AllocationEndIP 定义可分配地址范围的结束地址。
	AllocationEndIP string `json:"allocationEndIp,omitempty"`
}

// AttachDeviceRequest 用于把设备挂载到指定子网。
type AttachDeviceRequest struct {
	// DeviceID 是要挂载到子网的设备 ID。
	DeviceID string `json:"deviceId"`
}

// Network 表示成员、权限与子网归属关系的逻辑分组。
// 注意：地址空间存在于子网层，而不是直接挂在网络层。
type Network struct {
	// NetworkID 是唯一的逻辑网络标识。
	NetworkID string `json:"networkId"`
	// Name 是网络展示名称。
	Name string `json:"name"`
	// Description 是运维说明或业务描述。
	Description string `json:"description,omitempty"`
	// DefaultSubnetID 是创建网络时自动生成的默认子网 ID。
	DefaultSubnetID string `json:"defaultSubnetId,omitempty"`
	// DefaultSubnetCIDR 为 phase 1 UI 流程提供便捷返回。
	DefaultSubnetCIDR string `json:"defaultSubnetCidr,omitempty"`
}

// Subnet 表示网络内实际承载地址空间的容器。
type Subnet struct {
	// SubnetID 是唯一子网标识。
	SubnetID string `json:"subnetId"`
	// NetworkID 是所属父级逻辑网络的 ID。
	NetworkID string `json:"networkId"`
	// Name 是子网展示名称。
	Name string `json:"name"`
	// CIDR 是子网地址段。
	CIDR string `json:"cidr"`
	// GatewayIP 是预留网关或路由 IP。
	GatewayIP string `json:"gatewayIp,omitempty"`
	// AllocationStartIP 是可分配范围中的起始 IP。
	AllocationStartIP string `json:"allocationStartIp,omitempty"`
	// AllocationEndIP 是可分配范围中的结束 IP。
	AllocationEndIP string `json:"allocationEndIp,omitempty"`
	// IsDefault 标记该子网是否为 phase-1 加入流程自动创建的默认子网。
	IsDefault bool `json:"isDefault"`
	// Status 是子网生命周期状态的粗粒度表示。
	Status string `json:"status,omitempty"`
}

// JoinNetworkRequest 用于让设备加入网络。
// Phase 1 注意：服务端会自动把设备挂载到默认子网。
type JoinNetworkRequest struct {
	// DeviceID 是要加入目标网络的设备 ID。
	DeviceID string `json:"deviceId"`
}

// NetworkMember 描述网络层级的成员关系。
// 注意：这里不携带虚拟 IP，因为 IP 归属于子网挂载关系。
type NetworkMember struct {
	// MemberID 是网络成员关系的唯一标识。
	MemberID string `json:"memberId"`
	// NetworkID 是所属网络 ID。
	NetworkID string `json:"networkId"`
	// DeviceID 是对应的成员设备 ID。
	DeviceID string `json:"deviceId"`
	// Role 是网络层级角色，例如 owner 或 member。
	Role string `json:"role"`
	// Status 是成员关系的生命周期状态。
	Status string `json:"status,omitempty"`
}

// SubnetAttachment 描述设备在网络中的子网落点及其分配到的 IP。
type SubnetAttachment struct {
	// AttachmentID 是子网挂载关系的唯一标识。
	AttachmentID string `json:"attachmentId"`
	// NetworkID 是所属网络 ID。
	NetworkID string `json:"networkId"`
	// SubnetID 是设备挂载到的子网 ID。
	SubnetID string `json:"subnetId"`
	// DeviceID 是被挂载的设备 ID。
	DeviceID string `json:"deviceId"`
	// VirtualIP 是在该子网内分配给设备的虚拟 IP。
	VirtualIP string `json:"virtualIp,omitempty"`
	// Status 是挂载关系的生命周期状态。
	Status string `json:"status,omitempty"`
}

// NetworkJoinResult 是 Join 操作的返回结果，包含成员关系和默认子网挂载结果。
type NetworkJoinResult struct {
	// Member 是新创建或已存在的网络层级成员关系。
	Member NetworkMember `json:"member"`
	// Attachment 是设备落到默认子网后的挂载结果。
	Attachment SubnetAttachment `json:"attachment"`
}

// NetworkDetail 在网络基础信息上展开子网和成员列表。
type NetworkDetail struct {
	// Network 是基础网络元数据。
	Network
	// Subnets 列出该网络下当前定义的全部子网。
	Subnets []Subnet `json:"subnets"`
	// Members 列出该网络下全部网络层级成员。
	Members []NetworkMember `json:"members"`
}
