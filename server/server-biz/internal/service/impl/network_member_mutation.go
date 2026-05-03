package impl

import (
	"context"
	"fmt"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s dbNetworkService) UpdateMemberStatus(userID, networkID, memberID string, req dto.UpdateNetworkMemberStatusRequest) (dto.NetworkMember, error) {
	status := strings.TrimSpace(req.Status)
	if status != "active" && status != "rejected" {
		return dto.NetworkMember{}, fmt.Errorf("%w: status must be active or rejected", ErrInvalidArgument)
	}
	ctx := context.Background()
	record, err := s.state.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.NetworkMember{}, ErrNotFound
		}
		return dto.NetworkMember{}, err
	}
	if record.OwnerUserID != userID {
		return dto.NetworkMember{}, ErrForbidden
	}
	member, err := s.state.pg.GetMemberByID(ctx, memberID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.NetworkMember{}, ErrNotFound
		}
		return dto.NetworkMember{}, err
	}
	if member.NetworkID != networkID {
		return dto.NetworkMember{}, ErrNotFound
	}
	if member.Role == "owner" && status != "active" {
		return dto.NetworkMember{}, fmt.Errorf("%w: owner membership cannot be rejected", ErrInvalidArgument)
	}
	var memberDevice repo.Device
	if status == "active" && member.Role != "owner" {
		memberDevice, err = s.state.pg.GetDeviceByID(ctx, member.DeviceID)
		if err != nil {
			return dto.NetworkMember{}, err
		}
		if err := s.state.ensureFixedDeviceLimitAllowsActivation(ctx, record.OwnerUserID, networkID, record.DefaultSubnetID, member.DeviceID); err != nil {
			return dto.NetworkMember{}, err
		}
	}
	if err := s.state.pg.UpdateMemberStatus(ctx, memberID, status); err != nil {
		return dto.NetworkMember{}, err
	}
	member.Status = status
	if status == "active" && member.Role != "owner" {
		s.state.publishActiveNetworkEnabled(memberDevice.UserID, networkID, "network join approved")
	}
	if status == "rejected" {
		if err := s.state.cleanupRejectedNetworkMember(ctx, networkID, member); err != nil {
			return dto.NetworkMember{}, err
		}
	}
	return member, nil
}

func (s dbNetworkService) InviteMember(userID, networkID string, req dto.InviteNetworkMemberRequest) (dto.NetworkMember, error) {
	email := strings.TrimSpace(req.Email)
	if email == "" {
		return dto.NetworkMember{}, fmt.Errorf("%w: email is required", ErrInvalidArgument)
	}
	ctx := context.Background()
	record, err := s.state.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.NetworkMember{}, ErrNotFound
		}
		return dto.NetworkMember{}, err
	}
	if record.OwnerUserID != userID {
		return dto.NetworkMember{}, ErrForbidden
	}
	invited, err := s.state.pg.GetUserByEmail(ctx, email)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.NetworkMember{}, ErrNotFound
		}
		return dto.NetworkMember{}, err
	}
	devices, err := s.state.pg.ListDevicesByUser(ctx, invited.UserID)
	if err != nil {
		return dto.NetworkMember{}, err
	}
	if len(devices) == 0 {
		return dto.NetworkMember{}, fmt.Errorf("%w: invited user has no registered device", ErrInvalidArgument)
	}
	return s.requestMemberForNetwork(ctx, invited.UserID, devices[0].DeviceID, record)
}

func (s *dbState) cleanupRejectedNetworkMember(ctx context.Context, networkID string, member dto.NetworkMember) error {
	if member.Role == "owner" {
		return nil
	}
	if err := s.cleanupDeactivatedNetworkDevice(ctx, networkID, member.DeviceID); err != nil {
		return err
	}
	return s.clearDeviceOwnerActiveNetworkIfNoActiveMembership(ctx, networkID, member.DeviceID)
}

func (s *dbState) clearDeviceOwnerActiveNetworkIfNoActiveMembership(ctx context.Context, networkID, deviceID string) error {
	device, err := s.pg.GetDeviceByID(ctx, deviceID)
	if err != nil {
		return err
	}
	user, err := s.pg.GetUserByID(ctx, device.UserID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(user.ActiveNetworkID) == networkID {
		keepActive := false
		devices, err := s.pg.ListDevicesByUser(ctx, device.UserID)
		if err != nil {
			return err
		}
		for _, ownedDevice := range devices {
			candidate, err := s.pg.GetMemberByNetworkDevice(ctx, networkID, ownedDevice.DeviceID)
			if err == nil && candidate.Status == "active" {
				keepActive = true
				break
			}
		}
		if !keepActive {
			if err := s.pg.UpdateUserActiveNetwork(ctx, device.UserID, ""); err != nil {
				return err
			}
		}
	}
	return nil
}
