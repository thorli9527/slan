package service

import "github.com/slan/server/server-biz/api/dto"

// Network 定义逻辑网络、子网与挂载关系的编排能力。
type Network interface {
	// Create 创建一个新的逻辑网络并将请求用户设为拥有者。
	// 当前约束：同一用户只能拥有一个自建网络。
	Create(userID string, req dto.CreateNetworkRequest) (dto.Network, error)
	// Home 返回当前用户的活动网络和自有宿主网络摘要。
	Home(userID string) (dto.NetworkHome, error)
	// List 返回当前用户可见的网络列表。
	List(userID string) ([]dto.Network, error)
	// Update 修改当前用户自有网络的默认网段，并触发成员虚拟 IP 重分配。
	Update(userID, networkID string, req dto.UpdateNetworkRequest) (dto.Network, error)
	// UpdateDNS 修改网络级 DNS 配置；仅 owner 可访问。
	UpdateDNS(userID, networkID string, req dto.UpdateNetworkDNSRequest) (dto.NetworkDetail, error)
	// UpdateJoinKey 设置或清空网络加入 key；仅 owner 可访问。
	UpdateJoinKey(userID, networkID string, req dto.UpdateNetworkJoinKeyRequest) (dto.NetworkDetail, error)
	// Get 返回某个网络的完整详情，包括成员、子网和挂载关系视图。
	Get(userID, networkID string) (dto.NetworkDetail, error)
	// Join 将指定设备加入网络，并返回加入结果及必要的挂载信息。
	// 当前约束：一个设备只能同时属于一个活跃网络，加入新网络会切换离开旧网络。
	Join(userID, networkID string, req dto.JoinNetworkRequest) (dto.NetworkJoinResult, error)
	// JoinByOwnerEmail 允许当前用户按宿主邮箱定位网络并加入。
	JoinByOwnerEmail(userID string, req dto.JoinNetworkByOwnerEmailRequest) (dto.NetworkJoinByOwnerEmailResult, error)
	// JoinByKey 允许当前用户按网络加入 key 接入目标网络。
	JoinByKey(userID string, req dto.JoinNetworkByKeyRequest) (dto.NetworkJoinResult, error)
	// Switch 将当前用户切换到某个已拥有访问权限的目标网络。
	Switch(userID, networkID string, req dto.SwitchNetworkRequest) (dto.NetworkJoinResult, error)
	// Activate 在目标网络上为设备建立子网挂载并分配虚拟 IP。
	Activate(userID, networkID string, req dto.JoinNetworkRequest) (dto.NetworkJoinResult, error)
	// Deactivate 释放设备在目标网络上的子网挂载与虚拟 IP，但保留成员关系。
	Deactivate(userID, networkID string, req dto.DeactivateNetworkRequest) error
	// ListMembers 返回网络当前的成员设备列表。
	ListMembers(userID, networkID string) ([]dto.NetworkMember, error)
	// UpdateMemberStatus 允许网络 owner 审批或拒绝加入申请。
	UpdateMemberStatus(userID, networkID, memberID string, req dto.UpdateNetworkMemberStatusRequest) (dto.NetworkMember, error)
	// ListAssignments 返回网络内设备与虚拟 IP 的绑定关系；仅网络 owner 可访问。
	ListAssignments(userID, networkID string) ([]dto.NetworkAssignment, error)
	// CreateSubnet 在指定网络内创建一个新的子网。
	CreateSubnet(userID, networkID string, req dto.CreateSubnetRequest) (dto.Subnet, error)
	// ListSubnets 返回指定网络下的所有子网。
	ListSubnets(userID, networkID string) ([]dto.Subnet, error)
	// AttachDevice 将设备挂载到指定子网，并在需要时分配虚拟 IP。
	AttachDevice(userID, networkID, subnetID string, req dto.AttachDeviceRequest) (dto.SubnetAttachment, error)
	// UpdateAttachmentIP 允许网络 owner 手动修改设备挂载的虚拟 IP。
	UpdateAttachmentIP(userID, networkID, attachmentID string, req dto.UpdateAttachmentIPRequest) (dto.SubnetAttachment, error)
	// UpdateAttachmentRemark 允许网络 owner 修改网络内设备备注，或设备所有者维护自己的备注。
	UpdateAttachmentRemark(userID, networkID, attachmentID string, req dto.UpdateAttachmentRemarkRequest) (dto.NetworkAssignment, error)
}

// Allocator 定义服务端 DHCP 风格的虚拟 IP 分配能力。
// 注意：IP 分配绑定的是子网挂载关系，而不是设备实体本身。
type Allocator interface {
	// Allocate 为指定挂载关系自动分配一个可用的虚拟 IP。
	Allocate(networkID, subnetID, attachmentID, deviceID string) (string, error)
	// Reserve 为指定挂载关系保留调用方显式提供的虚拟 IP。
	Reserve(networkID, subnetID, attachmentID, deviceID, ip string) (string, error)
	// Release 释放某个挂载关系当前占用的虚拟 IP。
	Release(attachmentID string) error
}
