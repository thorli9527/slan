package impl

import (
	"context"
	"fmt"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

func (s *dbState) membershipRole(ctx context.Context, networkID, deviceID, ownerUserID, userID string) (string, error) {
	if ownerUserID != userID {
		return "member", nil
	}
	members, err := s.pg.ListMembersByNetwork(ctx, networkID)
	if err != nil {
		return "", err
	}
	for _, member := range members {
		if member.Role == "owner" && member.DeviceID != deviceID {
			return "member", nil
		}
	}
	return "owner", nil
}

// ensureNetworkAccess 校验用户是否对目标网络拥有读取和加入能力。
//
// 访问规则分为两类：
// 1. 网络所有者天然拥有全部访问权限。
// 2. 非所有者只要其任一设备已经加入该网络，也允许继续读取网络与成员视图。
func (s *dbState) ensureNetworkAccess(ctx context.Context, userID, networkID string) error {
	record, err := s.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if record.OwnerUserID == userID {
		return nil
	}

	devices, err := s.pg.ListDevicesByUser(ctx, userID)
	if err != nil {
		return err
	}
	for _, device := range devices {
		if _, err := s.pg.GetMemberByNetworkDevice(ctx, networkID, device.DeviceID); err == nil {
			return nil
		} else if !repo.IsNotFound(err) {
			return err
		}
	}
	return ErrForbidden
}

func (s *dbState) requireActiveNetworkMember(ctx context.Context, networkID, deviceID string, missingErr error, label string) (dto.NetworkMember, error) {
	member, err := s.pg.GetMemberByNetworkDevice(ctx, networkID, deviceID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.NetworkMember{}, fmt.Errorf("%w: %s is not a network member", missingErr, label)
		}
		return dto.NetworkMember{}, err
	}
	if member.Status != "active" {
		return dto.NetworkMember{}, fmt.Errorf("%w: %s membership is not active", ErrForbidden, label)
	}
	return member, nil
}

// ensureMember 确保设备已经在网络中拥有 member 记录。
//
// 该 helper 让 Join 流程保持幂等：如果成员已存在则直接复用，
// 否则按默认角色创建一条激活中的成员记录。
func (s *dbState) ensureMember(ctx context.Context, networkID, deviceID, role string) (dto.NetworkMember, error) {
	return s.ensureMemberWithStatus(ctx, networkID, deviceID, role, "active")
}

func (s *dbState) ensureMemberWithStatus(ctx context.Context, networkID, deviceID, role, status string) (dto.NetworkMember, error) {
	member, err := s.pg.GetMemberByNetworkDevice(ctx, networkID, deviceID)
	if err == nil {
		if member.Status == "rejected" && status == "pending" {
			if err := s.pg.UpdateMemberStatus(ctx, member.MemberID, status); err != nil {
				return dto.NetworkMember{}, err
			}
			member.Status = status
		}
		return member, nil
	}
	if !repo.IsNotFound(err) {
		return dto.NetworkMember{}, err
	}

	member = dto.NetworkMember{
		MemberID:  util.NewID("member"),
		NetworkID: networkID,
		DeviceID:  deviceID,
		Role:      role,
		CreatedAt: time.Now().Unix(),
		Status:    status,
	}
	return member, s.pg.CreateMember(ctx, member)
}

// ensureSingleNetworkMembership keeps one device attached to exactly one
// active network at a time.
//
// When a device joins a new network, older memberships, subnet attachments, and
// control sessions in other networks are removed so the device effectively
// switches to the target network.
func (s *dbState) ensureSingleNetworkMembership(ctx context.Context, deviceID, networkID string) error {
	if err := s.pg.DeleteControlSessionsByDeviceExceptNetwork(ctx, deviceID, networkID); err != nil {
		return err
	}
	if err := s.pg.DeleteAttachmentsByDeviceExceptNetwork(ctx, deviceID, networkID); err != nil {
		return err
	}
	return s.pg.DeleteMembersByDeviceExceptNetwork(ctx, deviceID, networkID)
}

func (s *dbState) ensureSingleActiveNetworkForUser(ctx context.Context, userID, networkID string) error {
	devices, err := s.pg.ListDevicesByUser(ctx, userID)
	if err != nil {
		return err
	}
	for _, device := range devices {
		if err := s.ensureSingleNetworkMembership(ctx, device.DeviceID, networkID); err != nil {
			return err
		}
	}
	return s.pg.UpdateUserActiveNetwork(ctx, userID, networkID)
}

// ensureAttachment 确保设备已经挂载到指定子网并由服务端 DHCP 分配虚拟地址。
//
// 对调用方来说，这也是一个幂等操作：如果已有挂载记录则原样返回，
// 如果不存在则分配新的虚拟 IP 并创建一条 active attachment。
func (s *dbState) ensureAttachment(ctx context.Context, networkID, subnetID, deviceID string) (dto.SubnetAttachment, error) {
	attachment, err := s.pg.GetAttachmentBySubnetDevice(ctx, subnetID, deviceID)
	if err == nil {
		return attachment, nil
	}
	if !repo.IsNotFound(err) {
		return dto.SubnetAttachment{}, err
	}

	subnet, err := s.pg.GetSubnetByID(ctx, subnetID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.SubnetAttachment{}, ErrNotFound
		}
		return dto.SubnetAttachment{}, err
	}

	virtualIP, err := s.allocateIP(ctx, subnet)
	if err != nil {
		return dto.SubnetAttachment{}, err
	}
	if member, err := s.pg.GetMemberByNetworkDevice(ctx, networkID, deviceID); err == nil && member.Role == "owner" {
		if preferred, ok, err := s.preferredOwnerIP(ctx, subnet); err != nil {
			return dto.SubnetAttachment{}, err
		} else if ok {
			virtualIP = preferred
		}
	}

	attachment = dto.SubnetAttachment{
		AttachmentID: util.NewID("att"),
		NetworkID:    networkID,
		SubnetID:     subnetID,
		DeviceID:     deviceID,
		VirtualIP:    virtualIP,
		Status:       "active",
	}
	return attachment, s.pg.CreateAttachment(ctx, attachment)
}

func (s *dbState) requireActiveNetworkAttachment(ctx context.Context, networkID, deviceID string, missingErr error, label string) (dto.SubnetAttachment, error) {
	attachments, err := s.pg.ListAttachmentsByDevice(ctx, deviceID)
	if err != nil {
		return dto.SubnetAttachment{}, err
	}
	for _, attachment := range attachments {
		if attachment.NetworkID != networkID {
			continue
		}
		if attachment.Status == "active" && attachment.VirtualIP != "" {
			return attachment, nil
		}
	}
	return dto.SubnetAttachment{}, fmt.Errorf("%w: %s has no active network attachment", missingErr, label)
}
