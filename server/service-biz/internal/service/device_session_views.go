package service

import (
	"context"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
	"github.com/slan/service-biz/internal/repository"
)

func deviceSessionView(item model.DeviceSession) DeviceSessionView {
	return DeviceSessionView{
		SessionID:     item.SessionID,
		DeviceID:      item.DeviceID,
		CredentialID:  item.CredentialID,
		AccessToken:   item.AccessToken,
		RefreshToken:  item.RefreshToken,
		Status:        item.Status,
		SessionMode:   item.SessionMode,
		ExpiresAt:     item.ExpiresAt,
		RefreshExpiry: item.RefreshExpiry,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
		RevokedAt:     item.RevokedAt,
	}
}

func buildBoundDeviceSessionView(
	ctx context.Context,
	networks repository.NetworkRepository,
	mqttConfig mqttkit.Config,
	now time.Time,
	device model.Device,
	session model.DeviceSession,
) (DeviceSessionBoundView, error) {
	profile, err := buildDeviceProfile(ctx, networks, device)
	if err != nil {
		return DeviceSessionBoundView{}, err
	}
	deviceNetworks, err := activeDeviceNetworks(ctx, networks, device.DeviceID)
	if err != nil {
		return DeviceSessionBoundView{}, err
	}
	return DeviceSessionBoundView{
		Profile: profile,
		Session: deviceSessionView(session),
		MQTT:    newDeviceMQTTProfile(mqttConfig, now, device.DeviceID, session.CredentialID, session.RefreshExpiry, deviceNetworks),
	}, nil
}
