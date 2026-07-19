package service

import (
	"context"
	"strings"

	"github.com/slan/service-biz/internal/model"
)

func (s DeviceSessionService) AuthenticateDeviceSession(ctx context.Context, accessToken string) (DeviceSessionView, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return DeviceSessionView{}, ErrUnauthorized
	}
	session, ok, err := s.Devices.GetDeviceSessionByAccessToken(ctx, accessToken)
	if err != nil {
		return DeviceSessionView{}, err
	}
	now := deviceNow(s.Now).Unix()
	if !ok || session.Status != tokenStatusActive || session.RevokedAt > 0 || session.ExpiresAt < now {
		return DeviceSessionView{}, ErrUnauthorized
	}
	return deviceSessionView(session), nil
}

func (s DeviceSessionService) BindDeviceSession(ctx context.Context, input BindDeviceSessionInput) (DeviceSessionBoundView, error) {
	input = normalizeBindDeviceSessionInput(input)
	if input.DeviceID == "" {
		return DeviceSessionBoundView{}, ErrInvalidArgument
	}
	device, err := s.resolveBindDevice(ctx, input)
	if err != nil {
		return DeviceSessionBoundView{}, err
	}
	nowUnix := deviceNow(s.Now).Unix()
	device, updated := applyBindDeviceSessionInput(device, input, nowUnix)
	if !managedDeviceVirtualIP(device.VirtualIP) {
		device.VirtualIP = allocatedDeviceVirtualIP(newDeviceVirtualIPID(s.Devices))
		device.UpdatedAt = nowUnix
		updated = true
	}
	if updated {
		if err := s.Devices.SaveDevice(ctx, device); err != nil {
			return DeviceSessionBoundView{}, err
		}
	}
	session, err := newManagedDeviceSession(deviceNow(s.Now), s.NewSessID, input.DeviceID, input.SessionMode)
	if err != nil {
		return DeviceSessionBoundView{}, err
	}
	if err := replaceDeviceSession(ctx, s.Devices, session); err != nil {
		return DeviceSessionBoundView{}, err
	}
	return buildBoundDeviceSessionView(ctx, s.Users, s.Networks, s.MQTT, deviceNow(s.Now), device, session)
}

func (s DeviceSessionService) resolveBindDevice(ctx context.Context, input BindDeviceSessionInput) (model.Device, error) {
	device, err := getManagedDevice(ctx, s.Devices, input.DeviceID)
	if err == nil {
		if input.UserID != "" && device.OwnerID != input.UserID {
			return model.Device{}, ErrForbidden
		}
		return device, nil
	}
	if err != ErrNotFound {
		return model.Device{}, err
	}
	if input.UserID == "" {
		return model.Device{}, ErrNotFound
	}
	if input.Name == "" || input.Platform == "" {
		return model.Device{}, ErrInvalidArgument
	}
	return registerManagedDevice(ctx, s.Users, s.Devices, s.Networks, s.Now, nil, RegisterDeviceInput{
		OwnerID:       input.UserID,
		ActorUserID:   input.UserID,
		DeviceID:      input.DeviceID,
		Name:          input.Name,
		Platform:      input.Platform,
		Alias:         input.Alias,
		OSName:        input.OSName,
		OSVersion:     input.OSVersion,
		PublicKey:     input.PublicKey,
		DeviceVersion: input.DeviceVersion,
		CountryCode:   input.CountryCode,
	})
}

func (s DeviceSessionService) RenewDeviceSession(ctx context.Context, accessToken string, input RenewDeviceSessionInput) (DeviceSessionBoundView, error) {
	input = normalizeRenewDeviceSessionInput(input)
	if input.RefreshToken == "" {
		return DeviceSessionBoundView{}, ErrInvalidArgument
	}
	now := deviceNow(s.Now)
	session, ok, err := s.Devices.GetDeviceSessionByRefreshToken(ctx, input.RefreshToken)
	if err != nil {
		return DeviceSessionBoundView{}, err
	}
	if !ok || session.Status != tokenStatusActive || session.RevokedAt > 0 || session.RefreshExpiry < now.Unix() {
		return DeviceSessionBoundView{}, ErrUnauthorized
	}
	if accessToken != "" && normalizeDeviceAccessToken(accessToken) != session.AccessToken {
		return DeviceSessionBoundView{}, ErrUnauthorized
	}
	session, err = newManagedDeviceSession(now, s.NewSessID, session.DeviceID, session.SessionMode)
	if err != nil {
		return DeviceSessionBoundView{}, err
	}
	if err := replaceDeviceSession(ctx, s.Devices, session); err != nil {
		return DeviceSessionBoundView{}, err
	}
	device, err := getManagedDevice(ctx, s.Devices, session.DeviceID)
	if err != nil {
		return DeviceSessionBoundView{}, err
	}
	nowUnix := now.Unix()
	device, updated := applyRenewDeviceSessionInput(device, input, nowUnix)
	if updated {
		if err := s.Devices.SaveDevice(ctx, device); err != nil {
			return DeviceSessionBoundView{}, err
		}
	}
	return buildBoundDeviceSessionView(ctx, s.Users, s.Networks, s.MQTT, now, device, session)
}
