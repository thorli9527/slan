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
	// BindDeviceID 允许在创建网络成功后立即把当前设备绑定到默认子网。
	BindDeviceID string `json:"bindDeviceId,omitempty"`
}

// UpdateNetworkRequest 用于修改用户自有网络的默认网段。
type UpdateNetworkRequest struct {
	// Name 允许一并修改网络展示名；为空时保留原值。
	Name string `json:"name,omitempty"`
	// Description 允许一并修改网络说明；为空时保留原值。
	Description string `json:"description,omitempty"`
	// CIDR 是新的默认子网 CIDR。
	CIDR string `json:"cidr"`
}

// UpdateNetworkDNSRequest 用于修改网络级 DNS 配置。
type UpdateNetworkDNSRequest struct {
	// Servers 是 DNS 服务器地址列表。
	Servers []string `json:"servers,omitempty"`
	// SearchDomains 是 DNS 搜索域列表。
	SearchDomains []string `json:"searchDomains,omitempty"`
}

// UpdateNetworkJoinKeyRequest 用于设置或清空网络加入 key。
type UpdateNetworkJoinKeyRequest struct {
	// JoinKey 是可选的显式 key；为空时服务端会生成新的 32 位随机 key。
	JoinKey string `json:"joinKey,omitempty"`
}

// SwitchNetworkRequest 用于显式切换当前活动网络。
type SwitchNetworkRequest struct {
	// DeviceID 是切换到目标网络时承载当前用户网络关系的设备 ID。
	DeviceID string `json:"deviceId"`
}

// DeactivateNetworkRequest 用于显式停用当前设备在目标网络上的本地接入。
type DeactivateNetworkRequest struct {
	// DeviceID 是要停用的设备 ID。
	DeviceID string `json:"deviceId"`
}

// UpdateNetworkMemberStatusRequest 用于 owner 审批或拒绝网络加入申请。
type UpdateNetworkMemberStatusRequest struct {
	// Status 是目标成员状态，当前支持 active 或 rejected。
	Status string `json:"status"`
}

// JoinNetworkByOwnerEmailRequest 允许用户按宿主邮箱加入其网络。
type JoinNetworkByOwnerEmailRequest struct {
	// OwnerEmail 是目标网络宿主用户的邮箱。
	OwnerEmail string `json:"ownerEmail"`
	// DeviceID 是当前用户要加入目标网络的设备 ID。
	DeviceID string `json:"deviceId"`
}

// JoinNetworkByKeyRequest 允许用户按网络加入 key 接入目标网络。
type JoinNetworkByKeyRequest struct {
	// JoinKey 是目标网络 owner 设置的加入 key。
	JoinKey string `json:"joinKey"`
	// DeviceID 是当前用户要加入目标网络的设备 ID。
	DeviceID string `json:"deviceId"`
}

// UpdateAttachmentIPRequest 允许网络 owner 手动调整某个设备的虚拟 IP。
type UpdateAttachmentIPRequest struct {
	// VirtualIP 是目标设备在该网络中的新虚拟 IP。
	VirtualIP string `json:"virtualIp"`
}

// UpdateAttachmentRemarkRequest 允许网络 owner 调整网络内设备备注。
type UpdateAttachmentRemarkRequest struct {
	// Remark 是网络内显示的备注。
	Remark string `json:"remark,omitempty"`
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
	// JoinKeyConfigured 标记当前网络是否已经定义了加入 key。
	JoinKeyConfigured bool `json:"joinKeyConfigured,omitempty"`
}

// NetworkHome 汇总当前用户的活动网络和自有宿主网络。
type NetworkHome struct {
	// ActiveNetwork 是当前用户唯一活动中的网络。
	ActiveNetwork *Network `json:"activeNetwork,omitempty"`
	// OwnedNetwork 是当前用户自建并拥有的网络；没有时为空。
	OwnedNetwork *Network `json:"ownedNetwork,omitempty"`
	// HasNetwork 表示当前用户是否已经拥有或加入任意网络。
	HasNetwork bool `json:"hasNetwork"`
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
// Phase 1 注意：服务端会自动把设备挂载到默认子网，并由服务端 DHCP 分配虚拟 IP。
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
	// CreatedAt 是成员关系创建时间。
	CreatedAt int64 `json:"createdAt,omitempty"`
	// Status 是成员关系的生命周期状态。
	Status string `json:"status,omitempty"`
}

// SubnetAttachment 描述设备在网络中的子网落点及其服务端 DHCP 租约。
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
	// Remark 是网络 owner 维护的设备备注。
	Remark string `json:"remark,omitempty"`
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

// NetworkJoinByOwnerEmailResult 返回按邮箱加入别人网络后的网络与挂载结果。
type NetworkJoinByOwnerEmailResult struct {
	// Network 是按邮箱解析到的目标宿主网络。
	Network Network `json:"network"`
	// Member 是当前设备在目标网络中的成员关系。
	Member NetworkMember `json:"member"`
	// Attachment 是当前设备加入默认子网后的挂载结果。
	Attachment SubnetAttachment `json:"attachment"`
}

// NetworkDetail 在网络基础信息上展开子网和成员列表。
type NetworkDetail struct {
	// Network 是基础网络元数据。
	Network
	// OwnedByCurrentUser 标记当前请求用户是否为网络 owner。
	OwnedByCurrentUser bool `json:"ownedByCurrentUser"`
	// DNS 是网络级 DNS 配置。
	DNS DNSConfig `json:"dns"`
	// JoinKey 是当前网络 owner 可见的加入 key；非 owner 请求时为空。
	JoinKey string `json:"joinKey,omitempty"`
	// Subnets 列出该网络下当前定义的全部子网。
	Subnets []Subnet `json:"subnets"`
	// Members 列出该网络下全部网络层级成员。
	Members []NetworkMember `json:"members"`
}

// NetworkAssignment 展示网络内设备、用户和虚拟 IP 的绑定关系。
type NetworkAssignment struct {
	// AttachmentID 是该设备在网络中的子网挂载关系 ID。
	AttachmentID string `json:"attachmentId"`
	// NetworkID 是所属网络 ID。
	NetworkID string `json:"networkId"`
	// SubnetID 是所属子网 ID。
	SubnetID string `json:"subnetId"`
	// DeviceID 是设备 ID。
	DeviceID string `json:"deviceId"`
	// DeviceName 是设备展示名。
	DeviceName string `json:"deviceName"`
	// UserID 是设备所属用户 ID。
	UserID string `json:"userId"`
	// UserEmail 是设备所属用户邮箱。
	UserEmail string `json:"userEmail"`
	// Role 是该设备在网络中的角色。
	Role string `json:"role"`
	// Remark 是网络 owner 维护的设备备注。
	Remark string `json:"remark,omitempty"`
	// VirtualIP 是当前分配到该设备的虚拟 IP。
	VirtualIP string `json:"virtualIp,omitempty"`
	// Status 是挂载状态。
	Status string `json:"status,omitempty"`
}
