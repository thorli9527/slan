package impl

import (
	"context"

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
	if err := s.state.ensureDeviceProvisionedInActiveNetwork(ctx, userID, record.DeviceID); err != nil {
		return dto.Device{}, err
	}
	return s.state.buildDeviceDTO(ctx, record), nil
}

func (s dbDeviceService) ListByUser(userID string) ([]dto.Device, error) {
	ctx := context.Background()
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
	networkDevices, err := s.state.pg.ListDevicesByNetworks(ctx, networkIDs)
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
	seen := make(map[string]struct{})
	var out []string
	for _, member := range members {
		if _, ok := seen[member.NetworkID]; ok {
			continue
		}
		seen[member.NetworkID] = struct{}{}
		out = append(out, member.NetworkID)
	}
	return out, nil
}
