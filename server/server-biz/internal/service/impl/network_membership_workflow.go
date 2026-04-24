package impl

import (
	"context"
	"fmt"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s dbNetworkService) requireDeviceID(deviceID string) error {
	if strings.TrimSpace(deviceID) == "" {
		return fmt.Errorf("%w: deviceId is required", ErrInvalidArgument)
	}
	return nil
}

func (s dbNetworkService) loadNetworkForDevice(ctx context.Context, userID, networkID, deviceID string, allowOwnerWithoutMembership bool) (repo.Network, error) {
	record, err := s.state.pg.GetNetworkByID(ctx, networkID)
	if err != nil {
		if repo.IsNotFound(err) {
			return repo.Network{}, ErrNotFound
		}
		return repo.Network{}, err
	}
	if err := s.state.ensureDeviceOwner(ctx, userID, deviceID); err != nil {
		return repo.Network{}, err
	}
	if err := s.state.ensureNetworkAccess(ctx, userID, networkID); err != nil {
		if !allowOwnerWithoutMembership || record.OwnerUserID != userID {
			return repo.Network{}, err
		}
	}
	return record, nil
}

func (s dbNetworkService) ensureActiveMemberForNetwork(ctx context.Context, userID, deviceID string, record repo.Network) (dto.NetworkMember, error) {
	if err := s.state.ensureSingleActiveNetworkForUser(ctx, userID, record.NetworkID); err != nil {
		return dto.NetworkMember{}, err
	}
	role, err := s.state.membershipRole(ctx, record.NetworkID, deviceID, record.OwnerUserID, userID)
	if err != nil {
		return dto.NetworkMember{}, err
	}
	return s.state.ensureMember(ctx, record.NetworkID, deviceID, role)
}

func (s dbNetworkService) requestMemberForNetwork(ctx context.Context, userID, deviceID string, record repo.Network) (dto.NetworkMember, error) {
	role, err := s.state.membershipRole(ctx, record.NetworkID, deviceID, record.OwnerUserID, userID)
	if err != nil {
		return dto.NetworkMember{}, err
	}
	status := "pending"
	if record.OwnerUserID == userID {
		status = "active"
		if err := s.state.ensureSingleActiveNetworkForUser(ctx, userID, record.NetworkID); err != nil {
			return dto.NetworkMember{}, err
		}
	}
	return s.state.ensureMemberWithStatus(ctx, record.NetworkID, deviceID, role, status)
}

func (s dbNetworkService) lookupOwnedNetworkByOwnerEmail(ctx context.Context, ownerEmail string) (repo.Network, error) {
	owner, err := s.state.pg.GetUserByEmail(ctx, ownerEmail)
	if err != nil {
		if repo.IsNotFound(err) {
			return repo.Network{}, ErrNotFound
		}
		return repo.Network{}, err
	}
	target, err := s.state.pg.GetOwnedNetworkByUser(ctx, owner.UserID)
	if err != nil {
		if repo.IsNotFound(err) {
			return repo.Network{}, ErrNotFound
		}
		return repo.Network{}, err
	}
	return target, nil
}
