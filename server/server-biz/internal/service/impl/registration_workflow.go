package impl

import (
	"context"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/util"
)

func (s dbDeviceService) upsertDeviceRecord(ctx context.Context, userID string, req dto.RegisterDeviceRequest) (repo.Device, error) {
	deviceID := strings.TrimSpace(req.DeviceID)
	if deviceID == "" {
		deviceID = util.NewID("dev")
	}
	record := repo.Device{
		DeviceID:      deviceID,
		UserID:        userID,
		Name:          req.Name,
		Platform:      req.Platform,
		DeviceVersion: strings.TrimSpace(req.DeviceVersion),
		CountryCode:   normalizeRelayCountryCode(req.CountryCode),
		Status:        "offline",
		PublicKey:     &req.PublicKey,
		CreatedAt:     time.Now().Unix(),
	}
	if strings.TrimSpace(req.DeviceID) != "" {
		current, err := s.state.pg.GetDeviceByID(ctx, deviceID)
		if err == nil {
			if current.UserID != userID && strings.TrimSpace(current.UserID) != "" && strings.TrimSpace(userID) != "" {
				return repo.Device{}, ErrForbidden
			}
			if strings.TrimSpace(current.UserID) != "" && strings.TrimSpace(userID) == "" {
				record.UserID = current.UserID
			}
			record.CreatedAt = current.CreatedAt
			if err := s.state.pg.UpdateDevice(ctx, record); err != nil {
				return repo.Device{}, err
			}
			return s.state.pg.GetDeviceByID(ctx, deviceID)
		}
		if !repo.IsNotFound(err) {
			return repo.Device{}, err
		}
		if err := s.state.pg.InsertDevice(ctx, record); err != nil {
			if !repo.IsUniqueViolation(err) {
				return repo.Device{}, err
			}
			current, err := s.state.pg.GetDeviceByID(ctx, deviceID)
			if err != nil {
				return repo.Device{}, err
			}
			if current.UserID != userID && strings.TrimSpace(current.UserID) != "" && strings.TrimSpace(userID) != "" {
				return repo.Device{}, ErrForbidden
			}
			return current, nil
		}
		return record, nil
	}
	if err := s.state.pg.InsertDevice(ctx, record); err != nil {
		if !repo.IsUniqueViolation(err) {
			return repo.Device{}, err
		}
		current, err := s.state.pg.GetDeviceByID(ctx, deviceID)
		if err != nil {
			return repo.Device{}, err
		}
		if current.UserID != userID {
			return repo.Device{}, ErrForbidden
		}
		return current, nil
	}
	return record, nil
}

func (s dbNodeService) buildNodeDTO(ctx context.Context, req dto.RegisterNodeRequest) dto.Node {
	node := dto.Node{
		NodeID:        req.NodeID,
		DeviceID:      req.DeviceID,
		NodePublicKey: req.NodePublicKey,
		Capabilities:  append([]string(nil), req.Capabilities...),
	}
	node.NetworkIDs, _ = s.state.activeDeviceNetworkIDs(ctx, req.DeviceID)
	return node
}
