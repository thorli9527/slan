package impl

import (
	"context"
	"fmt"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s dbNetworkService) Home(userID string) (dto.NetworkHome, error) {
	ctx := context.Background()
	var home dto.NetworkHome

	if owned, err := s.state.pg.GetOwnedNetworkByUser(ctx, userID); err == nil {
		record := owned.ToDTO()
		home.OwnedNetwork = &record
		home.HasNetwork = true
	} else if !repo.IsNotFound(err) {
		return dto.NetworkHome{}, err
	}

	items, err := s.List(userID)
	if err != nil {
		return dto.NetworkHome{}, err
	}
	if len(items) > 0 {
		record := items[0]
		home.ActiveNetwork = &record
		home.HasNetwork = true
	}
	return home, nil
}

func (s dbNetworkService) List(userID string) ([]dto.Network, error) {
	records, err := s.state.pg.ListVisibleNetworksByUser(context.Background(), userID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.Network, 0, len(records))
	for _, record := range records {
		out = append(out, record.ToDTO())
	}
	return out, nil
}

func (s dbNetworkService) Get(userID, networkID string) (dto.NetworkDetail, error) {
	ctx := context.Background()
	record, err := s.state.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.NetworkDetail{}, ErrNotFound
		}
		return dto.NetworkDetail{}, err
	}
	if err := s.state.ensureNetworkAccess(ctx, userID, networkID); err != nil {
		return dto.NetworkDetail{}, err
	}
	subnets, err := s.state.pg.ListSubnetsByNetwork(ctx, networkID)
	if err != nil {
		return dto.NetworkDetail{}, err
	}
	members, err := s.state.pg.ListMembersByNetwork(ctx, networkID)
	if err != nil {
		return dto.NetworkDetail{}, err
	}
	return dto.NetworkDetail{
		Network:            record.ToDTO(),
		OwnedByCurrentUser: record.OwnerUserID == userID,
		DNS:                record.DNSConfig(),
		JoinKey:            visibleJoinKey(record, userID),
		Subnets:            subnets,
		Members:            members,
	}, nil
}

func (s dbNetworkService) ListMembers(userID, networkID string) ([]dto.NetworkMember, error) {
	ctx := context.Background()
	if err := s.state.ensureNetworkAccess(ctx, userID, networkID); err != nil {
		return nil, err
	}
	return s.state.pg.ListMembersByNetwork(ctx, networkID)
}

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
	if err := s.state.pg.UpdateMemberStatus(ctx, memberID, status); err != nil {
		return dto.NetworkMember{}, err
	}
	member.Status = status
	if status == "active" && member.Role != "owner" {
		device, err := s.state.pg.GetDeviceByID(ctx, member.DeviceID)
		if err != nil {
			return dto.NetworkMember{}, err
		}
		s.state.publishActiveNetworkEnabled(device.UserID, networkID, "network join approved")
	}
	if status == "rejected" {
		if err := s.state.cleanupRejectedNetworkMember(ctx, networkID, member); err != nil {
			return dto.NetworkMember{}, err
		}
	}
	return member, nil
}

func (s *dbState) cleanupRejectedNetworkMember(ctx context.Context, networkID string, member dto.NetworkMember) error {
	if member.Role == "owner" {
		return nil
	}
	if err := s.cleanupDeactivatedNetworkDevice(ctx, networkID, member.DeviceID); err != nil {
		return err
	}
	device, err := s.pg.GetDeviceByID(ctx, member.DeviceID)
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

func (s dbNetworkService) ListAssignments(userID, networkID string) ([]dto.NetworkAssignment, error) {
	ctx := context.Background()
	record, err := s.state.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if record.OwnerUserID != userID {
		return nil, ErrForbidden
	}
	return s.state.pg.ListAssignmentsByNetwork(ctx, networkID)
}

func (s dbNetworkService) ListSubnets(userID, networkID string) ([]dto.Subnet, error) {
	ctx := context.Background()
	if err := s.state.ensureNetworkAccess(ctx, userID, networkID); err != nil {
		return nil, err
	}
	return s.state.pg.ListSubnetsByNetwork(ctx, networkID)
}
