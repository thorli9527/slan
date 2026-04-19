package service

import "github.com/slan/server/server-biz/api/dto"

// Network 定义逻辑网络、子网与挂载关系的编排能力。
type Network interface {
	// Create 创建一个新的逻辑网络并将请求用户设为拥有者。
	Create(userID string, req dto.CreateNetworkRequest) (dto.Network, error)
	// List 返回当前用户可见的网络列表。
	List(userID string) ([]dto.Network, error)
	// Get 返回某个网络的完整详情，包括成员、子网和挂载关系视图。
	Get(userID, networkID string) (dto.NetworkDetail, error)
	// Join 将指定设备加入网络，并返回加入结果及必要的挂载信息。
	Join(userID, networkID string, req dto.JoinNetworkRequest) (dto.NetworkJoinResult, error)
	// ListMembers 返回网络当前的成员设备列表。
	ListMembers(userID, networkID string) ([]dto.NetworkMember, error)
	// CreateSubnet 在指定网络内创建一个新的子网。
	CreateSubnet(userID, networkID string, req dto.CreateSubnetRequest) (dto.Subnet, error)
	// ListSubnets 返回指定网络下的所有子网。
	ListSubnets(userID, networkID string) ([]dto.Subnet, error)
	// AttachDevice 将设备挂载到指定子网，并在需要时分配虚拟 IP。
	AttachDevice(userID, networkID, subnetID string, req dto.AttachDeviceRequest) (dto.SubnetAttachment, error)
}

// Allocator 定义子网范围内的虚拟 IP 分配能力。
// 注意：IP 分配绑定的是子网挂载关系，而不是设备实体本身。
type Allocator interface {
	// Allocate 为指定挂载关系自动分配一个可用的虚拟 IP。
	Allocate(networkID, subnetID, attachmentID, deviceID string) (string, error)
	// Reserve 为指定挂载关系保留调用方显式提供的虚拟 IP。
	Reserve(networkID, subnetID, attachmentID, deviceID, ip string) (string, error)
	// Release 释放某个挂载关系当前占用的虚拟 IP。
	Release(attachmentID string) error
}
