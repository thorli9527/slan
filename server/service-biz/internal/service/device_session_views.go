package service

import (
	"context"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
	"github.com/slan/service-biz/internal/repository"
)

func buildBoundDeviceSessionView(
	ctx context.Context,
	users repository.UserRepository,
	networks repository.NetworkRepository,
	mqttConfig mqttkit.Config,
	now time.Time,
	device model.Device,
	session model.DeviceSession,
) (DeviceSessionBoundView, error) {
	profile, err := buildDeviceProfile(ctx, users, networks, device)
	if err != nil {
		return DeviceSessionBoundView{}, err
	}
	deviceNetworks, err := networks.ListNetworksByDevice(ctx, device.DeviceID)
	if err != nil {
		return DeviceSessionBoundView{}, err
	}
	return DeviceSessionBoundView{
		Profile: profile,
		Session: deviceSessionView(session),
		MQTT:    newDeviceMQTTProfile(mqttConfig, now, device.DeviceID, deviceNetworks),
	}, nil
}

func buildBootstrapDeviceSessionView(
	ctx context.Context,
	users repository.UserRepository,
	networks repository.NetworkRepository,
	mqttConfig mqttkit.Config,
	now time.Time,
	device model.Device,
	session model.DeviceSession,
) (DeviceSessionBootstrapView, error) {
	bound, err := buildBoundDeviceSessionView(ctx, users, networks, mqttConfig, now, device, session)
	if err != nil {
		return DeviceSessionBootstrapView{}, err
	}
	return DeviceSessionBootstrapView{
		Profile:    bound.Profile,
		Device:     deviceView(device),
		Session:    bound.Session,
		MQTT:       bound.MQTT,
		Credential: bound.MQTT.Credential,
	}, nil
}
