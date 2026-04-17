package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
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
		DeviceID:  newID("dev"),
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
