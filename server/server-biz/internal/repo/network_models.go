package repo

import "github.com/slan/server/server-biz/api/dto"

// Network 是逻辑网络的持久化模型。
type Network struct {
	// NetworkID 是网络唯一标识。
	NetworkID string `gorm:"column:network_id;primaryKey"`
	// OwnerUserID 是网络拥有者用户 ID。
	OwnerUserID string `gorm:"column:owner_user_id;index;not null"`
	// Name 是网络展示名称。
	Name string `gorm:"column:name;not null"`
	// Description 是网络说明。
	Description string `gorm:"column:description;not null;default:''"`
	// DefaultSubnetID 是默认子网 ID。
	DefaultSubnetID string `gorm:"column:default_subnet_id;not null"`
	// DefaultSubnetCIDR 是默认子网 CIDR。
	DefaultSubnetCIDR string `gorm:"column:default_subnet_cidr;not null"`
}

func (Network) TableName() string { return "networks" }

func (m Network) ToDTO() dto.Network {
	return dto.Network{
		NetworkID:         m.NetworkID,
		Name:              m.Name,
		Description:       m.Description,
		DefaultSubnetID:   m.DefaultSubnetID,
		DefaultSubnetCIDR: m.DefaultSubnetCIDR,
	}
}

// Subnet 是逻辑网络下实际承载地址空间的持久化模型。
type Subnet struct {
	// SubnetID 是子网唯一标识。
	SubnetID string `gorm:"column:subnet_id;primaryKey"`
	// NetworkID 是所属网络 ID。
	NetworkID string `gorm:"column:network_id;index;not null"`
	// Name 是子网名称。
	Name string `gorm:"column:name;not null"`
	// CIDR 是子网地址段。
	CIDR string `gorm:"column:cidr;not null"`
	// GatewayIP 是网关地址。
	GatewayIP string `gorm:"column:gateway_ip;not null;default:''"`
	// AllocationStartIP 是可分配地址起点。
	AllocationStartIP string `gorm:"column:allocation_start_ip;not null;default:''"`
	// AllocationEndIP 是可分配地址终点。
	AllocationEndIP string `gorm:"column:allocation_end_ip;not null;default:''"`
	// IsDefault 标记是否为默认子网。
	IsDefault bool `gorm:"column:is_default;not null;default:false"`
	// Status 是子网状态。
	Status string `gorm:"column:status;not null;default:'active'"`
}

func (Subnet) TableName() string { return "subnets" }

func (m Subnet) ToDTO() dto.Subnet {
	return dto.Subnet{
		SubnetID:          m.SubnetID,
		NetworkID:         m.NetworkID,
		Name:              m.Name,
		CIDR:              m.CIDR,
		GatewayIP:         m.GatewayIP,
		AllocationStartIP: m.AllocationStartIP,
		AllocationEndIP:   m.AllocationEndIP,
		IsDefault:         m.IsDefault,
		Status:            m.Status,
	}
}

// NetworkMember 描述设备加入网络后的成员关系记录。
type NetworkMember struct {
	// MemberID 是成员关系唯一标识。
	MemberID string `gorm:"column:member_id;primaryKey"`
	// NetworkID 是所属网络。
	NetworkID string `gorm:"column:network_id;index;not null;uniqueIndex:idx_network_device"`
	// DeviceID 是成员设备 ID。
	DeviceID string `gorm:"column:device_id;index;not null;uniqueIndex:idx_network_device"`
	// Role 是网络层级角色。
	Role string `gorm:"column:role;not null"`
	// Status 是成员状态。
	Status string `gorm:"column:status;not null;default:'active'"`
}

func (NetworkMember) TableName() string { return "network_members" }

func (m NetworkMember) ToDTO() dto.NetworkMember {
	return dto.NetworkMember{
		MemberID:  m.MemberID,
		NetworkID: m.NetworkID,
		DeviceID:  m.DeviceID,
		Role:      m.Role,
		Status:    m.Status,
	}
}

// SubnetAttachment 描述设备落到某个子网后的挂载关系与分配 IP。
type SubnetAttachment struct {
	// AttachmentID 是挂载关系唯一标识。
	AttachmentID string `gorm:"column:attachment_id;primaryKey"`
	// NetworkID 是所属网络。
	NetworkID string `gorm:"column:network_id;index;not null"`
	// SubnetID 是目标子网。
	SubnetID string `gorm:"column:subnet_id;index;not null;uniqueIndex:idx_subnet_device"`
	// DeviceID 是被挂载设备。
	DeviceID string `gorm:"column:device_id;index;not null;uniqueIndex:idx_subnet_device"`
	// VirtualIP 是分配给该挂载关系的虚拟 IP。
	VirtualIP string `gorm:"column:virtual_ip;not null;default:'';uniqueIndex:idx_subnet_ip"`
	// Status 是挂载状态。
	Status string `gorm:"column:status;not null;default:'active'"`
}

func (SubnetAttachment) TableName() string { return "subnet_attachments" }

func (m SubnetAttachment) ToDTO() dto.SubnetAttachment {
	return dto.SubnetAttachment{
		AttachmentID: m.AttachmentID,
		NetworkID:    m.NetworkID,
		SubnetID:     m.SubnetID,
		DeviceID:     m.DeviceID,
		VirtualIP:    m.VirtualIP,
		Status:       m.Status,
	}
}
