package impl

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	if err := s.state.requireAttachmentAvailableForActivation(ctx, record.DefaultSubnetID, req.DeviceID); err != nil {
		return dto.NetworkJoinResult{}, err
	}
	attachment, err := s.state.requireActiveNetworkAttachment(ctx, networkID, req.DeviceID, ErrForbidden, "device")
	if err != nil {
		return dto.NetworkJoinResult{}, err
	}
	networkMap, err := s.relayNetworkMapForDevice(ctx, userID, networkID, req.DeviceID)
	if err != nil {
		return dto.NetworkJoinResult{}, err
	}
	s.state.refreshEnabledNetworkMember(ctx, networkID, req.DeviceID, attachment.VirtualIP, time.Now())
	s.state.publishDeviceNetworkPresence(userID, networkID, req.DeviceID, attachment.VirtualIP, true, "device network activated")
	return dto.NetworkJoinResult{Member: member, Attachment: attachment, NetworkMap: networkMap}, nil
}

func (s dbNetworkService) RelayCandidates(userID, networkID string, req dto.JoinNetworkRequest) (dto.NetworkMap, error) {
	if err := s.requireDeviceID(req.DeviceID); err != nil {
		return dto.NetworkMap{}, err
	}
	ctx := context.Background()
	record, err := s.loadNetworkForDevice(ctx, userID, networkID, req.DeviceID, false)
	if err != nil {
		return dto.NetworkMap{}, err
	}
	if _, err := s.state.requireActiveNetworkMember(ctx, networkID, req.DeviceID, ErrForbidden, "device"); err != nil {
		return dto.NetworkMap{}, err
	}
	if err := s.state.requireAttachmentAvailableForActivation(ctx, record.DefaultSubnetID, req.DeviceID); err != nil {
		return dto.NetworkMap{}, err
	}
	if _, err := s.state.requireActiveNetworkAttachment(ctx, networkID, req.DeviceID, ErrForbidden, "device"); err != nil {
		return dto.NetworkMap{}, err
	}
	return s.relayNetworkMapForDevice(ctx, userID, networkID, req.DeviceID)
}

func (s dbNetworkService) relayNetworkMapForDevice(ctx context.Context, userID, networkID, deviceID string) (dto.NetworkMap, error) {
	self := dto.Node{DeviceID: deviceID}
	if nodes, err := s.state.pg.ListNodesByDevice(ctx, deviceID); err == nil && len(nodes) > 0 {
		self = nodes[0].ToDTO(nil)
	}
	return s.state.buildNetworkMap(ctx, userID, self, networkID), nil
}

func (s *dbState) requireAttachmentAvailableForActivation(ctx context.Context, subnetID, deviceID string) error {
	attachment, err := s.pg.GetAttachmentBySubnetDevice(ctx, subnetID, deviceID)
	if err != nil {
		if repo.IsNotFound(err) {
			return nil
		}
		return err
	}
	switch strings.ToLower(strings.TrimSpace(attachment.Status)) {
	case "disabled", "suspended":
		return fmt.Errorf("%w: device unavailable, contact administrator", ErrForbidden)
	default:
		return nil
	}
}

func (s *dbState) ensureFixedDeviceLimitAllowsActivation(ctx context.Context, ownerUserID, networkID, subnetID, deviceID string) error {
	limit := s.effectivePlanConfig(ctx, ownerUserID).MaxActiveDevices
	if attachment, err := s.pg.GetAttachmentBySubnetDevice(ctx, subnetID, deviceID); err == nil && attachment.Status == "active" && attachment.VirtualIP != "" {
		return nil
	} else if err != nil && !repo.IsNotFound(err) {
		return err
	}
	activeCount, err := s.pg.CountActiveAttachmentsByNetwork(ctx, networkID)
	if err != nil {
		return err
	}
	if activeCount >= int64(limit) {
		return fmt.Errorf("%w: free version allows at most %d active devices; download the product and deploy it yourself for more devices", ErrDeviceLimitExceeded, limit)
	}
	return nil
}

func (s dbNetworkService) Deactivate(userID, networkID string, req dto.DeactivateNetworkRequest) error {
	if err := s.requireDeviceID(req.DeviceID); err != nil {
		return err
	}
	ctx := context.Background()
	if _, err := s.loadNetworkForDevice(ctx, userID, networkID, req.DeviceID, false); err != nil {
		return err
	}
	if _, err := s.state.requireActiveNetworkMember(ctx, networkID, req.DeviceID, ErrForbidden, "device"); err != nil {
		return err
	}
	if err := s.state.markDeviceNetworkRuntimeOffline(ctx, networkID, req.DeviceID); err != nil {
		return err
	}
	s.state.publishDeviceNetworkPresence(userID, networkID, req.DeviceID, "", false, "device network deactivated")
	return nil
}
