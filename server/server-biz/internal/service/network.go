package service

import "github.com/slan/server/server-biz/api/dto"

// Network defines orchestration capabilities for logical networks, subnets, and attachments.
type Network interface {
	Create(userID string, req dto.CreateNetworkRequest) (dto.Network, error)
	Home(userID string) (dto.NetworkHome, error)
	List(userID string) ([]dto.Network, error)
	Update(userID, networkID string, req dto.UpdateNetworkRequest) (dto.Network, error)
	UpdateDNS(userID, networkID string, req dto.UpdateNetworkDNSRequest) (dto.NetworkDetail, error)
	UpdateJoinKey(userID, networkID string, req dto.UpdateNetworkJoinKeyRequest) (dto.NetworkDetail, error)
	Get(userID, networkID string) (dto.NetworkDetail, error)
	Join(userID, networkID string, req dto.JoinNetworkRequest) (dto.NetworkJoinResult, error)
	JoinByKey(userID string, req dto.JoinNetworkByKeyRequest) (dto.NetworkJoinResult, error)
	Switch(userID, networkID string, req dto.SwitchNetworkRequest) (dto.NetworkJoinResult, error)
	Activate(userID, networkID string, req dto.JoinNetworkRequest) (dto.NetworkJoinResult, error)
	Deactivate(userID, networkID string, req dto.DeactivateNetworkRequest) error
	PlanStatus(userID string) (dto.PlanStatus, error)
	ListMembers(userID, networkID string) ([]dto.NetworkMember, error)
	UpdateMemberStatus(userID, networkID, memberID string, req dto.UpdateNetworkMemberStatusRequest) (dto.NetworkMember, error)
	InviteMember(userID, networkID string, req dto.InviteNetworkMemberRequest) (dto.NetworkMember, error)
	ListAssignments(userID, networkID string) ([]dto.NetworkAssignment, error)
	ListSubnets(userID, networkID string) ([]dto.Subnet, error)
	UpdateAttachmentIP(userID, networkID, attachmentID string, req dto.UpdateAttachmentIPRequest) (dto.SubnetAttachment, error)
	UpdateAttachmentRemark(userID, networkID, attachmentID string, req dto.UpdateAttachmentRemarkRequest) (dto.NetworkAssignment, error)
	UpdateAttachmentStatus(userID, networkID, attachmentID string, req dto.UpdateAttachmentStatusRequest) (dto.NetworkAssignment, error)
}

// Allocator defines server-side DHCP-style virtual IP allocation.
type Allocator interface {
	Allocate(networkID, subnetID, attachmentID, deviceID string) (string, error)
	Reserve(networkID, subnetID, attachmentID, deviceID, ip string) (string, error)
	Release(attachmentID string) error
}
