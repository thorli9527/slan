package impl

import (
	"context"
	"fmt"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/service"
	"github.com/slan/server/server-biz/internal/util"
)

type dbDeviceService struct{ state *dbState }
type dbNodeService struct{ state *dbState }

var (
	_ service.Device = dbDeviceService{}
	_ service.Node   = dbNodeService{}
)

func (s dbDeviceService) Register(userID string, req dto.RegisterDeviceRequest) (dto.Device, error) {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Platform) == "" || strings.TrimSpace(req.MachineID) == "" || strings.TrimSpace(req.PublicKey) == "" {
		return dto.Device{}, fmt.Errorf("%w: name, platform, machineId, and publicKey are required", ErrInvalidArgument)
	}
	ctx := context.Background()
	if _, err := s.state.pg.GetUserByID(ctx, userID); err != nil {
		if repo.IsNotFound(err) {
			return dto.Device{}, ErrUnauthorized
		}
		return dto.Device{}, err
	}
	record, err := s.state.pg.GetDeviceByUserMachine(ctx, userID, req.MachineID)
	if err == nil {
		record.Name = req.Name
		record.Platform = req.Platform
		record.PublicKey = &req.PublicKey
		networkIDs, _ := s.state.deviceNetworkIDs(ctx, record.DeviceID)
		if err := s.state.pg.UpdateDevice(ctx, record); err != nil {
			return dto.Device{}, err
		}
		return record.ToDTO(networkIDs), nil
	}
	if !repo.IsNotFound(err) {
		return dto.Device{}, err
	}

	record = repo.Device{
		DeviceID:  util.NewID("dev"),
		UserID:    userID,
		MachineID: req.MachineID,
		Name:      req.Name,
		Platform:  req.Platform,
		Status:    "offline",
		PublicKey: &req.PublicKey,
	}
	if err := s.state.pg.InsertDevice(ctx, record); err != nil {
		return dto.Device{}, err
	}
	return record.ToDTO([]string{}), nil
}

func (s dbDeviceService) ListByUser(userID string) ([]dto.Device, error) {
	ctx := context.Background()
	records, err := s.state.pg.ListDevicesByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]dto.Device, 0, len(records))
	for _, record := range records {
		networkIDs, _ := s.state.deviceNetworkIDs(ctx, record.DeviceID)
		out = append(out, record.ToDTO(networkIDs))
	}
	return out, nil
}

func (s dbNodeService) Register(userID string, req dto.RegisterNodeRequest) (dto.Node, error) {
	if strings.TrimSpace(req.DeviceID) == "" || strings.TrimSpace(req.NodeID) == "" || strings.TrimSpace(req.NodePublicKey) == "" {
		return dto.Node{}, fmt.Errorf("%w: deviceId, nodeId, and nodePublicKey are required", ErrInvalidArgument)
	}
	ctx := context.Background()
	if err := s.state.ensureDeviceOwner(ctx, userID, req.DeviceID); err != nil {
		return dto.Node{}, err
	}
	node := dto.Node{
		NodeID:        req.NodeID,
		DeviceID:      req.DeviceID,
		NodePublicKey: req.NodePublicKey,
		Capabilities:  append([]string(nil), req.Capabilities...),
	}
	node.NetworkIDs, _ = s.state.deviceNetworkIDs(ctx, req.DeviceID)
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
	attachments, err := s.pg.ListAttachmentsByDevice(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	var out []string
	for _, attachment := range attachments {
		if _, ok := seen[attachment.NetworkID]; ok {
			continue
		}
		seen[attachment.NetworkID] = struct{}{}
		out = append(out, attachment.NetworkID)
	}
	return out, nil
}
