package impl

import (
	"context"
	"fmt"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s dbNetworkService) UpdateAttachmentIP(userID, networkID, attachmentID string, req dto.UpdateAttachmentIPRequest) (dto.SubnetAttachment, error) {
	ctx := context.Background()
	record, err := s.state.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.SubnetAttachment{}, ErrNotFound
		}
		return dto.SubnetAttachment{}, err
	}
	if record.OwnerUserID != userID {
		return dto.SubnetAttachment{}, ErrForbidden
	}
	attachment, err := s.state.pg.GetAttachmentByID(ctx, attachmentID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.SubnetAttachment{}, ErrNotFound
		}
		return dto.SubnetAttachment{}, err
	}
	if attachment.NetworkID != networkID {
		return dto.SubnetAttachment{}, ErrNotFound
	}
	if _, err := s.state.pg.GetMemberByNetworkDevice(ctx, networkID, attachment.DeviceID); err != nil {
		if repo.IsNotFound(err) {
			return dto.SubnetAttachment{}, ErrNotFound
		}
		return dto.SubnetAttachment{}, err
	}
	reserved, err := s.Reserve(networkID, attachment.SubnetID, attachment.AttachmentID, attachment.DeviceID, req.VirtualIP)
	if err != nil {
		return dto.SubnetAttachment{}, err
	}
	if err := s.state.pg.UpdateAttachmentVirtualIP(ctx, attachment.AttachmentID, reserved); err != nil {
		return dto.SubnetAttachment{}, err
	}
	attachment.VirtualIP = reserved
	s.state.publishDeviceIPReassigned(networkID, attachment.DeviceID, attachment.AttachmentID, reserved)
	s.state.publishNetworkRestartRequired(networkID, record.DefaultSubnetCIDR)
	return attachment, nil
}

func (s dbNetworkService) UpdateAttachmentRemark(userID, networkID, attachmentID string, req dto.UpdateAttachmentRemarkRequest) (dto.NetworkAssignment, error) {
	ctx := context.Background()
	record, err := s.state.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.NetworkAssignment{}, ErrNotFound
		}
		return dto.NetworkAssignment{}, err
	}
	attachment, err := s.state.pg.GetAttachmentByID(ctx, attachmentID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.NetworkAssignment{}, ErrNotFound
		}
		return dto.NetworkAssignment{}, err
	}
	if attachment.NetworkID != networkID {
		return dto.NetworkAssignment{}, ErrNotFound
	}
	if record.OwnerUserID != userID {
		if err := s.state.ensureDeviceOwner(ctx, userID, attachment.DeviceID); err != nil {
			return dto.NetworkAssignment{}, err
		}
	}
	remark := strings.TrimSpace(req.Remark)
	if err := s.state.pg.UpdateAttachmentRemark(ctx, attachmentID, remark); err != nil {
		return dto.NetworkAssignment{}, err
	}
	assignments, err := s.state.pg.ListAssignmentsByNetwork(ctx, networkID)
	if err != nil {
		return dto.NetworkAssignment{}, err
	}
	s.state.applyLiveDeviceNetworkStates(ctx, assignments)
	for _, item := range assignments {
		if item.AttachmentID == attachmentID {
			return item, nil
		}
	}
	return dto.NetworkAssignment{}, ErrNotFound
}

func (s dbNetworkService) UpdateAttachmentStatus(userID, networkID, attachmentID string, req dto.UpdateAttachmentStatusRequest) (dto.NetworkAssignment, error) {
	ctx := context.Background()
	record, err := s.state.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.NetworkAssignment{}, ErrNotFound
		}
		return dto.NetworkAssignment{}, err
	}
	if record.OwnerUserID != userID {
		return dto.NetworkAssignment{}, ErrForbidden
	}
	attachment, err := s.state.pg.GetAttachmentByID(ctx, attachmentID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.NetworkAssignment{}, ErrNotFound
		}
		return dto.NetworkAssignment{}, err
	}
	if attachment.NetworkID != networkID {
		return dto.NetworkAssignment{}, ErrNotFound
	}
	member, err := s.state.pg.GetMemberByNetworkDevice(ctx, networkID, attachment.DeviceID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.NetworkAssignment{}, ErrNotFound
		}
		return dto.NetworkAssignment{}, err
	}
	if member.Status != "active" {
		return dto.NetworkAssignment{}, fmt.Errorf("%w: device membership is not active", ErrForbidden)
	}

	status := strings.ToLower(strings.TrimSpace(req.Status))
	switch status {
	case "active", "enabled":
		if err := s.state.enableAttachment(ctx, record, attachment); err != nil {
			return dto.NetworkAssignment{}, err
		}
	case "disabled", "suspended":
		if err := s.state.disableAttachment(ctx, record, attachment); err != nil {
			return dto.NetworkAssignment{}, err
		}
	default:
		return dto.NetworkAssignment{}, fmt.Errorf("%w: status must be active or disabled", ErrInvalidArgument)
	}
	s.state.publishNetworkRestartRequired(networkID, record.DefaultSubnetCIDR)

	assignments, err := s.state.pg.ListAssignmentsByNetwork(ctx, networkID)
	if err != nil {
		return dto.NetworkAssignment{}, err
	}
	s.state.applyLiveDeviceNetworkStates(ctx, assignments)
	for _, item := range assignments {
		if item.AttachmentID == attachmentID {
			return item, nil
		}
	}
	return dto.NetworkAssignment{}, ErrNotFound
}

func (s *dbState) enableAttachment(ctx context.Context, network repo.Network, attachment dto.SubnetAttachment) error {
	if err := s.ensureFixedDeviceLimitAllowsActivation(ctx, network.OwnerUserID, network.NetworkID, attachment.SubnetID, attachment.DeviceID); err != nil {
		return err
	}
	virtualIP, err := s.ensureAttachmentVirtualIP(ctx, attachment)
	if err != nil {
		return err
	}
	if err := s.pg.UpdateAttachmentLease(ctx, attachment.AttachmentID, "active", virtualIP); err != nil {
		return err
	}
	if device, err := s.pg.GetDeviceByID(ctx, attachment.DeviceID); err == nil {
		_ = s.pg.UpdateUserActiveNetwork(ctx, device.UserID, network.NetworkID)
		s.publishActiveNetworkEnabled(device.UserID, network.NetworkID, "attachment enabled by network owner")
	}
	s.publishDeviceIPReassigned(network.NetworkID, attachment.DeviceID, attachment.AttachmentID, virtualIP)
	return nil
}

func (s *dbState) disableAttachment(ctx context.Context, network repo.Network, attachment dto.SubnetAttachment) error {
	virtualIP, err := s.ensureAttachmentVirtualIP(ctx, attachment)
	if err != nil {
		return err
	}
	s.publishDeviceIPReassigned(network.NetworkID, attachment.DeviceID, attachment.AttachmentID, "")
	s.publishDeviceNetworkDisabled(network.NetworkID, attachment.DeviceID, attachment.AttachmentID, "attachment disabled by network owner")
	if err := s.cleanupNetworkDeviceRuntime(ctx, network.NetworkID, attachment.DeviceID); err != nil {
		return err
	}
	if err := s.pg.UpdateAttachmentLease(ctx, attachment.AttachmentID, "disabled", virtualIP); err != nil {
		return err
	}
	if err := s.markDeviceNetworkAttachmentDisabled(ctx, network.NetworkID, attachment.DeviceID); err != nil {
		return err
	}
	if device, err := s.pg.GetDeviceByID(ctx, attachment.DeviceID); err == nil {
		_ = s.clearUserActiveNetworkIfNoAttachments(ctx, device.UserID, network.NetworkID)
	}
	return nil
}
