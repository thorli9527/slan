package service

import (
	"context"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

type TokenManagementService struct {
	Users    repository.UserRepository
	Sessions repository.UserSessionRepository
	Devices  repository.DeviceRepository
	Now      func() time.Time
}

func (s TokenManagementService) ListUserSessions(ctx context.Context, userID string) ([]UserManagedSessionView, error) {
	items, err := listUserSessionsByUserID(ctx, s.Sessions, normalizeUserID(userID))
	if err != nil {
		return nil, err
	}
	views := make([]UserManagedSessionView, 0, len(items))
	for _, item := range items {
		views = append(views, userManagedSessionView(item))
	}
	return views, nil
}

func (s TokenManagementService) RevokeUserSession(ctx context.Context, input RevokeUserManagedSessionInput) (UserManagedSessionView, error) {
	input = normalizeRevokeUserManagedSessionInput(input)
	if input.UserID == "" || input.ActorUserID == "" || input.SessionID == "" {
		return UserManagedSessionView{}, ErrInvalidArgument
	}
	if input.UserID != input.ActorUserID {
		return UserManagedSessionView{}, ErrUnauthorized
	}
	items, err := listUserSessionsByUserID(ctx, s.Sessions, input.UserID)
	if err != nil {
		return UserManagedSessionView{}, err
	}
	for _, item := range items {
		if item.SessionID != input.SessionID {
			continue
		}
		now := currentTimeSeconds(s.Now)
		item.Status = tokenStatusRevoked
		item.RevokedAt = now
		item.UpdatedAt = now
		if err := s.Sessions.SaveUserSession(ctx, item); err != nil {
			return UserManagedSessionView{}, err
		}
		return userManagedSessionView(item), nil
	}
	return UserManagedSessionView{}, ErrNotFound
}

func (s TokenManagementService) ListDeviceSessions(ctx context.Context, deviceID string) ([]DeviceManagedSessionView, error) {
	items, err := listDeviceSessionsByDeviceID(ctx, s.Devices, deviceID)
	if err != nil {
		return nil, err
	}
	views := make([]DeviceManagedSessionView, 0, len(items))
	for _, item := range items {
		views = append(views, deviceManagedSessionView(item))
	}
	return views, nil
}

func (s TokenManagementService) RevokeDeviceSession(ctx context.Context, input RevokeDeviceManagedSessionInput) (DeviceManagedSessionView, error) {
	input = normalizeRevokeDeviceManagedSessionInput(input)
	if input.DeviceID == "" || input.ActorUserID == "" || input.SessionID == "" {
		return DeviceManagedSessionView{}, ErrInvalidArgument
	}
	device, err := getManagedDevice(ctx, s.Devices, input.DeviceID)
	if err != nil {
		return DeviceManagedSessionView{}, err
	}
	if device.OwnerID != input.ActorUserID {
		return DeviceManagedSessionView{}, ErrUnauthorized
	}
	items, err := listDeviceSessionsByDeviceID(ctx, s.Devices, input.DeviceID)
	if err != nil {
		return DeviceManagedSessionView{}, err
	}
	for _, item := range items {
		if item.SessionID != input.SessionID {
			continue
		}
		now := currentTimeSeconds(s.Now)
		item.Status = tokenStatusRevoked
		item.RevokedAt = now
		item.UpdatedAt = now
		if err := s.Devices.SaveDeviceSession(ctx, item); err != nil {
			return DeviceManagedSessionView{}, err
		}
		return deviceManagedSessionView(item), nil
	}
	return DeviceManagedSessionView{}, ErrNotFound
}

func listUserSessionsByUserID(ctx context.Context, sessions repository.UserSessionRepository, userID string) ([]model.UserSession, error) {
	return sessions.ListUserSessionsByUserID(ctx, userID)
}

func listDeviceSessionsByDeviceID(ctx context.Context, devices repository.DeviceRepository, deviceID string) ([]model.DeviceSession, error) {
	return devices.ListDeviceSessionsByDeviceID(ctx, deviceID)
}

func currentTimeSeconds(now func() time.Time) int64 {
	if now != nil {
		return now().Unix()
	}
	return currentTime(nil).Unix()
}
