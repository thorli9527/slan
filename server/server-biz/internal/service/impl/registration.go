package impl

import (
	"context"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/service"
)

type dbDeviceService struct{ state *dbState }
type dbNodeService struct{ state *dbState }

var (
	_ service.Device = dbDeviceService{}
	_ service.Node   = dbNodeService{}
)

func (s dbDeviceService) Register(userID string, req dto.RegisterDeviceRequest) (dto.Device, error) {
	if err := s.requireRegisterDeviceRequest(req); err != nil {
		return dto.Device{}, err
	}
	ctx := context.Background()
	if err := s.state.requireUser(ctx, userID); err != nil {
		return dto.Device{}, err
	}
	record, err := s.upsertDeviceRecord(ctx, userID, req)
	if err != nil {
		return dto.Device{}, err
	}
	device := s.state.buildDeviceDTO(ctx, record)
	device.MQTT = s.state.buildDeviceMQTTCredential(record)
	return device, nil
}

func (s dbDeviceService) ListByUser(userID string) ([]dto.Device, error) {
	ctx := context.Background()
	s.state.cleanupExpiredControlPlaneState(ctx, time.Now())
	records, err := s.state.pg.ListDevicesByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	visibleNetworks, err := s.state.deviceListVisibleNetworks(ctx, userID)
	if err != nil {
		return nil, err
	}
	networkIDs := make([]string, 0, len(visibleNetworks))
	for _, network := range visibleNetworks {
		networkIDs = append(networkIDs, network.NetworkID)
	}
	ownedNetworkIDs := make([]string, 0, len(visibleNetworks))
	for _, network := range visibleNetworks {
		if network.OwnerUserID == userID {
			ownedNetworkIDs = append(ownedNetworkIDs, network.NetworkID)
		}
	}
	networkDevices, err := s.state.pg.ListDevicesByNetworks(ctx, ownedNetworkIDs)
	if err != nil {
		return nil, err
	}
	activeNetworkID := s.state.preferredDeviceListNetworkID(ctx, userID, visibleNetworks)
	seen := make(map[string]struct{}, len(records)+len(networkDevices))
	out := make([]dto.Device, 0, len(records)+len(networkDevices))
	for _, record := range records {
		seen[record.DeviceID] = struct{}{}
		out = append(out, s.state.buildDeviceDTOForNetwork(ctx, record, activeNetworkID))
	}
	for _, record := range networkDevices {
		if _, ok := seen[record.DeviceID]; ok {
			continue
		}
		seen[record.DeviceID] = struct{}{}
		out = append(out, s.state.buildDeviceDTOForNetwork(
			ctx,
			record,
			s.state.firstDeviceMembershipNetwork(ctx, record.DeviceID, networkIDs),
		))
	}
	return out, nil
}

func (s dbDeviceService) SetDeviceNetworkState(userID, deviceID, networkID string, req dto.DeviceNetworkStateRequest) (dto.DeviceNetworkState, error) {
	ctx := context.Background()
	record, err := s.state.pg.GetDeviceByID(ctx, deviceID)
	if err != nil {
		if repo.IsNotFound(err) {
			return dto.DeviceNetworkState{}, ErrNotFound
		}
		return dto.DeviceNetworkState{}, err
	}
	if record.UserID != userID {
		return dto.DeviceNetworkState{}, ErrForbidden
	}
	if err := s.state.ensureNetworkAccess(ctx, userID, networkID); err != nil {
		return dto.DeviceNetworkState{}, err
	}
	if _, err := s.state.requireActiveNetworkMember(ctx, networkID, deviceID, ErrForbidden, "device"); err != nil {
		return dto.DeviceNetworkState{}, err
	}
	if _, err := s.state.requireActiveNetworkAttachment(ctx, networkID, deviceID, ErrForbidden, "device"); err != nil {
		return dto.DeviceNetworkState{}, err
	}
	if err := s.state.upsertTrustedDeviceNetworkState(ctx, deviceID, networkID, req); err != nil {
		return dto.DeviceNetworkState{}, err
	}
	state, err := s.state.pg.GetDeviceNetworkState(ctx, deviceID, networkID)
	if err != nil {
		return dto.DeviceNetworkState{}, err
	}
	return state.ToDTO(), nil
}

func (s dbDeviceService) MarkMQTTReachable(deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return ErrInvalidArgument
	}
	ctx := context.Background()
	if _, err := s.state.pg.GetDeviceByID(ctx, deviceID); err != nil {
		if repo.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	networkIDs, err := s.state.activeDeviceNetworkIDs(ctx, deviceID)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	for _, networkID := range networkIDs {
		state, err := s.state.pg.GetDeviceNetworkState(ctx, deviceID, networkID)
		if err != nil {
			if !repo.IsNotFound(err) {
				return err
			}
			state = repo.DeviceNetworkState{
				DeviceID:  deviceID,
				NetworkID: networkID,
			}
		}
		state.ControlReachable = true
		state.LastSeenAt = now
		state.UpdatedAt = now
		if err := s.state.pg.UpsertDeviceNetworkState(ctx, state); err != nil {
			return err
		}
	}
	return nil
}

func (s dbNodeService) Register(userID string, req dto.RegisterNodeRequest) (dto.Node, error) {
	if err := s.requireRegisterNodeRequest(req); err != nil {
		return dto.Node{}, err
	}
	ctx := context.Background()
	if err := s.state.ensureDeviceOwner(ctx, userID, req.DeviceID); err != nil {
		return dto.Node{}, err
	}
	node := s.buildNodeDTO(ctx, req)
	if err := s.state.pg.UpsertNode(ctx, repo.Node{
		NodeID:        node.NodeID,
		UserID:        userID,
		DeviceID:      node.DeviceID,
		NodePublicKey: node.NodePublicKey,
		Capabilities:  node.Capabilities,
	}); err != nil {
		return dto.Node{}, err
	}
	return node, nil
}

func (s *dbState) ensureDeviceOwner(ctx context.Context, userID, deviceID string) error {
	record, err := s.pg.GetDeviceByID(ctx, deviceID)
	if err != nil {
		if repo.IsNotFound(err) {
			return ErrNotFound
		}
		return err
	}
	if record.UserID != userID {
		return ErrForbidden
	}
	return nil
}

func (s *dbState) deviceNetworkIDs(ctx context.Context, deviceID string) ([]string, error) {
	members, err := s.pg.ListMembersByDevice(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	return networkIDsForMembers(members, false), nil
}

func (s *dbState) activeDeviceNetworkIDs(ctx context.Context, deviceID string) ([]string, error) {
	members, err := s.pg.ListMembersByDevice(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	return networkIDsForMembers(members, true), nil
}

func networkIDsForMembers(members []dto.NetworkMember, activeOnly bool) []string {
	seen := make(map[string]struct{})
	var out []string
	for _, member := range members {
		if activeOnly && member.Status != "active" {
			continue
		}
		if _, ok := seen[member.NetworkID]; ok {
			continue
		}
		seen[member.NetworkID] = struct{}{}
		out = append(out, member.NetworkID)
	}
	return out
}
