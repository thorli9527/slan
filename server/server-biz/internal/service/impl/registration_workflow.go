package impl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

func (s dbDeviceService) requireRegisterDeviceRequest(req dto.RegisterDeviceRequest) error {
	if strings.TrimSpace(req.Name) == "" || strings.TrimSpace(req.Platform) == "" || strings.TrimSpace(req.MachineID) == "" || strings.TrimSpace(req.PublicKey) == "" {
		return fmt.Errorf("%w: name, platform, machineId, and publicKey are required", ErrInvalidArgument)
	}
	return nil
}

func (s dbNodeService) requireRegisterNodeRequest(req dto.RegisterNodeRequest) error {
	if strings.TrimSpace(req.DeviceID) == "" || strings.TrimSpace(req.NodeID) == "" || strings.TrimSpace(req.NodePublicKey) == "" {
		return fmt.Errorf("%w: deviceId, nodeId, and nodePublicKey are required", ErrInvalidArgument)
	}
	return nil
}

func (s *dbState) requireUser(ctx context.Context, userID string) error {
	if _, err := s.pg.GetUserByID(ctx, userID); err != nil {
		if repo.IsNotFound(err) {
			return ErrUnauthorized
		}
		return err
	}
	return nil
}

func (s dbDeviceService) upsertDeviceRecord(ctx context.Context, userID string, req dto.RegisterDeviceRequest) (repo.Device, error) {
	return s.state.pg.UpsertDeviceByUserMachine(ctx, repo.Device{
		DeviceID:  util.NewID("dev"),
		UserID:    userID,
		MachineID: req.MachineID,
		Name:      req.Name,
		Platform:  req.Platform,
		Status:    "offline",
		PublicKey: &req.PublicKey,
		CreatedAt: time.Now().Unix(),
	})
}

func (s *dbState) buildDeviceDTO(ctx context.Context, record repo.Device) dto.Device {
	networkIDs, _ := s.deviceNetworkIDs(ctx, record.DeviceID)
	device := record.ToDTO(networkIDs)
	user, err := s.pg.GetUserByID(ctx, record.UserID)
	if err != nil {
		return device
	}
	device.OwnerEmail = user.Email
	device.LinkStatus = record.Status
	activeNetworkID := strings.TrimSpace(user.ActiveNetworkID)
	if activeNetworkID == "" {
		return device
	}
	attachments, err := s.pg.ListAttachmentsByDevice(ctx, record.DeviceID)
	if err == nil {
		for _, attachment := range attachments {
			if attachment.NetworkID == activeNetworkID {
				device.CurrentVirtualIP = attachment.VirtualIP
				break
			}
		}
	}
	if member, err := s.pg.GetMemberByNetworkDevice(ctx, activeNetworkID, record.DeviceID); err == nil {
		device.JoinedAt = member.CreatedAt
	}
	if state, err := s.pg.GetLatestDeviceConnectionState(ctx, activeNetworkID, record.DeviceID); err == nil {
		if strings.TrimSpace(state.State) != "" {
			device.LinkStatus = state.State
		}
		device.ConnectivityProtocol = normalizeConnectivityProtocol(state.Path)
	}
	return device
}

func (s dbNodeService) buildNodeDTO(ctx context.Context, req dto.RegisterNodeRequest) dto.Node {
	node := dto.Node{
		NodeID:        req.NodeID,
		DeviceID:      req.DeviceID,
		NodePublicKey: req.NodePublicKey,
		Capabilities:  append([]string(nil), req.Capabilities...),
	}
	node.NetworkIDs, _ = s.state.deviceNetworkIDs(ctx, req.DeviceID)
	return node
}

func normalizeConnectivityProtocol(path string) string {
	value := strings.TrimSpace(path)
	switch value {
	case "":
		return ""
	case "derp":
		return "relay"
	default:
		return value
	}
}
