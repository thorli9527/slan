package network

import "github.com/slan/server/server-biz/api/dto"

// Service 定义逻辑网络、子网与挂载关系的编排能力。
// Phase 1 说明：
// - 每个网络创建时都会带一个默认子网
// - Join 会自动把设备挂载到该默认子网
type Service interface {
	// Create 一次性创建网络及其默认子网。
	Create(userID string, req dto.CreateNetworkRequest) (dto.Network, error)
	// List 返回该用户可见的全部网络。
	List(userID string) ([]dto.Network, error)
	// Get 返回网络详情，包含子网和网络层级成员。
	Get(userID, networkID string) (dto.NetworkDetail, error)
	// Join 把设备加入网络，并挂载到默认子网。
	Join(userID, networkID string, req dto.JoinNetworkRequest) (dto.NetworkJoinResult, error)
	// ListMembers 仅返回网络层级的成员关系。
	ListMembers(userID, networkID string) ([]dto.NetworkMember, error)
	// CreateSubnet 在目标网络下创建额外子网。
	CreateSubnet(userID, networkID string, req dto.CreateSubnetRequest) (dto.Subnet, error)
	// ListSubnets 返回目标网络下的全部子网。
	ListSubnets(userID, networkID string) ([]dto.Subnet, error)
	// AttachDevice 将已加入网络的设备挂载到指定子网。
	AttachDevice(userID, networkID, subnetID string, req dto.AttachDeviceRequest) (dto.SubnetAttachment, error)
}
