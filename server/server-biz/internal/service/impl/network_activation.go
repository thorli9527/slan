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
	return s.state.markDeviceNetworkRuntimeOffline(ctx, networkID, req.DeviceID)
}

func (s *dbState) markDeviceNetworkRuntimeOffline(ctx context.Context, networkID, deviceID string) error {
	return s.upsertTrustedDeviceNetworkState(ctx, deviceID, networkID, dto.DeviceNetworkStateRequest{
		ControlReachable: true,
		NetworkOnline:    false,
		TunnelUp:         false,
		LastProbeOK:      false,
	})
}

func (s *dbState) cleanupDeactivatedNetworkDevice(ctx context.Context, networkID, deviceID string) error {
	if err := s.cleanupNetworkDeviceRuntime(ctx, networkID, deviceID); err != nil {
		return err
	}
	return s.pg.DeleteAttachmentsByDeviceInNetwork(ctx, deviceID, networkID)
}

func (s *dbState) cleanupNetworkDeviceRuntime(ctx context.Context, networkID, deviceID string) error {
	nodes, err := s.pg.ListNodesByDevice(ctx, deviceID)
	if err != nil {
		return err
	}
	if sessions, err := s.pg.ListControlSessionsByDeviceInNetwork(ctx, deviceID, networkID); err != nil {
		return err
	} else {
		s.deleteControlSessionTokens(ctx, sessions)
	}
	if err := s.pg.DeleteControlSessionsByDeviceInNetwork(ctx, deviceID, networkID); err != nil {
		return err
	}
	_ = s.tokens.DeleteDeviceNetworkState(ctx, deviceID, networkID)
	for _, node := range nodes {
		s.publishPeerRemove(networkID, node.NodeID)
	}
	return nil
}

func (s *dbState) clearUserActiveNetworkIfNoAttachments(ctx context.Context, userID, networkID string) error {
	user, err := s.pg.GetUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if strings.TrimSpace(user.ActiveNetworkID) != networkID {
		return nil
	}
	devices, err := s.pg.ListDevicesByUser(ctx, userID)
	if err != nil {
		return err
	}
	for _, device := range devices {
		attachments, err := s.pg.ListAttachmentsByDevice(ctx, device.DeviceID)
		if err != nil {
			return err
		}
		for _, attachment := range attachments {
			if attachment.NetworkID == networkID && attachment.Status == "active" && attachment.VirtualIP != "" {
				return nil
			}
		}
	}
	return s.pg.UpdateUserActiveNetwork(ctx, userID, "")
}
