package impl

import (
	"context"
	"fmt"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
)

func (s dbNetworkService) Join(userID, networkID string, req dto.JoinNetworkRequest) (dto.NetworkJoinResult, error) {
	if err := s.requireDeviceID(req.DeviceID); err != nil {
		return dto.NetworkJoinResult{}, err
	}
	ctx := context.Background()
	record, err := s.loadNetworkForDevice(ctx, userID, networkID, req.DeviceID, true)
	if err != nil {
		return dto.NetworkJoinResult{}, err
	}
	member, err := s.requestMemberForNetwork(ctx, userID, req.DeviceID, record)
	if err != nil {
		return dto.NetworkJoinResult{}, err
	}
	return dto.NetworkJoinResult{Member: member}, nil
}

func (s dbNetworkService) JoinByOwnerEmail(userID string, req dto.JoinNetworkByOwnerEmailRequest) (dto.NetworkJoinByOwnerEmailResult, error) {
	if strings.TrimSpace(req.OwnerEmail) == "" {
		return dto.NetworkJoinByOwnerEmailResult{}, fmt.Errorf("%w: ownerEmail is required", ErrInvalidArgument)
	}
	if err := s.requireDeviceID(req.DeviceID); err != nil {
		return dto.NetworkJoinByOwnerEmailResult{}, err
	}
	ctx := context.Background()
	target, err := s.lookupOwnedNetworkByOwnerEmail(ctx, req.OwnerEmail)
	if err != nil {
		return dto.NetworkJoinByOwnerEmailResult{}, err
	}
	if err := s.state.ensureDeviceOwner(ctx, userID, req.DeviceID); err != nil {
		return dto.NetworkJoinByOwnerEmailResult{}, err
	}
	member, err := s.requestMemberForNetwork(ctx, userID, req.DeviceID, target)
	if err != nil {
		return dto.NetworkJoinByOwnerEmailResult{}, err
	}
	return dto.NetworkJoinByOwnerEmailResult{
		Network: target.ToDTO(),
		Member:  member,
	}, nil
}

func (s dbNetworkService) JoinByKey(userID string, req dto.JoinNetworkByKeyRequest) (dto.NetworkJoinResult, error) {
	joinKey := strings.TrimSpace(req.JoinKey)
	if joinKey == "" {
		return dto.NetworkJoinResult{}, fmt.Errorf("%w: joinKey is required", ErrInvalidArgument)
	}
	if err := s.requireDeviceID(req.DeviceID); err != nil {
		return dto.NetworkJoinResult{}, err
	}
	ctx := context.Background()
	if err := s.state.ensureDeviceOwner(ctx, userID, req.DeviceID); err != nil {
		return dto.NetworkJoinResult{}, err
	}
	target, err := s.state.pg.ConsumeNetworkByJoinKey(ctx, joinKey)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.NetworkJoinResult{}, ErrNotFound
		}
		return dto.NetworkJoinResult{}, err
	}
	member, err := s.requestMemberForNetwork(ctx, userID, req.DeviceID, target)
	if err != nil {
		return dto.NetworkJoinResult{}, err
	}
	return dto.NetworkJoinResult{Member: member}, nil
}

func (s dbNetworkService) Switch(userID, networkID string, req dto.SwitchNetworkRequest) (dto.NetworkJoinResult, error) {
	if err := s.requireDeviceID(req.DeviceID); err != nil {
		return dto.NetworkJoinResult{}, err
	}
	ctx := context.Background()
	record, err := s.loadNetworkForDevice(ctx, userID, networkID, req.DeviceID, false)
	if err != nil {
		return dto.NetworkJoinResult{}, err
	}
	member, err := s.ensureActiveMemberForNetwork(ctx, userID, req.DeviceID, record)
	if err != nil {
		return dto.NetworkJoinResult{}, err
	}
	return dto.NetworkJoinResult{Member: member}, nil
}

func (s dbNetworkService) Activate(userID, networkID string, req dto.JoinNetworkRequest) (dto.NetworkJoinResult, error) {
	if err := s.requireDeviceID(req.DeviceID); err != nil {
		return dto.NetworkJoinResult{}, err
	}
	ctx := context.Background()
	record, err := s.loadNetworkForDevice(ctx, userID, networkID, req.DeviceID, false)
	if err != nil {
		return dto.NetworkJoinResult{}, err
	}
	member, err := s.state.requireActiveNetworkMember(ctx, networkID, req.DeviceID, ErrForbidden, "device")
	if err != nil {
		return dto.NetworkJoinResult{}, err
	}
	if err := s.state.ensureSingleActiveNetworkForUser(ctx, userID, networkID); err != nil {
		return dto.NetworkJoinResult{}, err
	}
	attachment, err := s.state.ensureAttachment(ctx, networkID, record.DefaultSubnetID, req.DeviceID)
	if err != nil {
		return dto.NetworkJoinResult{}, err
	}
	return dto.NetworkJoinResult{Member: member, Attachment: attachment}, nil
}

func (s dbNetworkService) Deactivate(userID, networkID string, req dto.DeactivateNetworkRequest) error {
	if err := s.requireDeviceID(req.DeviceID); err != nil {
		return err
	}
	ctx := context.Background()
	if _, err := s.loadNetworkForDevice(ctx, userID, networkID, req.DeviceID, false); err != nil {
		return err
	}
	return s.state.pg.DeleteAttachmentsByDeviceInNetwork(ctx, req.DeviceID, networkID)
}
