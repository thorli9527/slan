package service

import (
	"context"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

func (s DeviceRuntimeAccessService) UpdateDeviceRuntime(ctx context.Context, input UpdateDeviceRuntimeInput) (DeviceProfileView, error) {
	device, err := s.updateDeviceRuntimeEntity(ctx, input)
	if err != nil {
		return DeviceProfileView{}, err
	}
	return buildDeviceProfile(ctx, s.Networks, device)
}

func (s DeviceRuntimeAccessService) updateDeviceRuntimeEntity(ctx context.Context, input UpdateDeviceRuntimeInput) (model.Device, error) {
	input = normalizeUpdateDeviceRuntimeInput(input)
	if input.DeviceID == "" {
		return model.Device{}, ErrInvalidArgument
	}
	nowTime := deviceNow(s.Now)
	now := nowTime.Unix()
	device, err := getManagedDevice(ctx, s.Devices, input.DeviceID)
	if err != nil {
		return model.Device{}, err
	}
	device = applyUpdateDeviceRuntime(device, input, now)
	if err := s.Devices.SaveDevice(ctx, device); err != nil {
		return model.Device{}, err
	}
	membershipUpdates, err := s.updateDeviceRuntimeMemberships(ctx, input, now)
	if err != nil {
		return model.Device{}, err
	}
	for _, update := range membershipUpdates {
		if !update.Publish {
			continue
		}
		if err := s.publishRuntimeMembershipEvent(ctx, device, update.NetworkID, update.Membership, nowTime); err != nil {
			return model.Device{}, err
		}
	}
	return device, nil
}

type deviceRuntimeMembershipUpdate struct {
	NetworkID  string
	Membership model.NetworkDevice
	Publish    bool
}

func (s DeviceRuntimeAccessService) updateDeviceRuntimeMemberships(ctx context.Context, input UpdateDeviceRuntimeInput, now int64) ([]deviceRuntimeMembershipUpdate, error) {
	if !hasDeviceRuntimeMembershipUpdate(input) {
		return nil, nil
	}
	networks, err := s.Networks.ListNetworksByDevice(ctx, input.DeviceID)
	if err != nil {
		return nil, err
	}
	reportedNetworkFound := input.NetworkID == ""
	updates := make([]deviceRuntimeMembershipUpdate, 0, len(networks))
	for _, network := range networks {
		membership, ok, err := findNetworkMembership(ctx, s.Networks, network.NetworkID, input.DeviceID)
		if err != nil {
			return nil, err
		}
		if !ok || !networkMemberActive(membership) {
			continue
		}
		if network.NetworkID == input.NetworkID {
			reportedNetworkFound = true
		}
		membership.NetworkID = network.NetworkID
		membership.DeviceID = input.DeviceID
		previous := membership
		updated := applyUpdateDeviceRuntimeMembership(membership, input, now)
		if err := s.Networks.SaveNetworkDevice(ctx, updated); err != nil {
			return nil, err
		}
		updates = append(updates, deviceRuntimeMembershipUpdate{
			NetworkID:  network.NetworkID,
			Membership: updated,
			Publish:    shouldPublishRuntimeMembershipPresenceEvent(previous, updated, time.Unix(now, 0)),
		})
	}
	if !reportedNetworkFound {
		return nil, ErrNotFound
	}
	return updates, nil
}

func (s DeviceRuntimeAccessService) RenewDevice(ctx context.Context, deviceID string) (DeviceProfileView, error) {
	device, err := s.renewDeviceEntity(ctx, deviceID)
	if err != nil {
		return DeviceProfileView{}, err
	}
	return buildDeviceProfile(ctx, s.Networks, device)
}

func (s DeviceRuntimeAccessService) renewDeviceEntity(ctx context.Context, deviceID string) (model.Device, error) {
	deviceID = normalizeDeviceID(deviceID)
	if deviceID == "" {
		return model.Device{}, ErrInvalidArgument
	}
	device, err := getManagedDevice(ctx, s.Devices, deviceID)
	if err != nil {
		return model.Device{}, err
	}
	device = renewManagedDevice(device, deviceNow(s.Now).Unix())
	if err := s.Devices.SaveDevice(ctx, device); err != nil {
		return model.Device{}, err
	}
	return device, nil
}

func (s DeviceRuntimeAccessService) DeviceNetworkConfigs(ctx context.Context, deviceID string) ([]NetworkSummaryView, error) {
	deviceID = normalizeDeviceID(deviceID)
	if deviceID == "" {
		return nil, ErrInvalidArgument
	}
	if _, err := getManagedDevice(ctx, s.Devices, deviceID); err != nil {
		return nil, err
	}
	items, err := s.Networks.ListNetworksByDevice(ctx, deviceID)
	if err != nil {
		return nil, err
	}
	return networkSummaryViews(items), nil
}

func (s DeviceRuntimeAccessService) DeviceMQTTCredential(ctx context.Context, deviceID, credentialID string, expiresAt int64) (*mqttkit.Credential, error) {
	deviceID = normalizeDeviceID(deviceID)
	if deviceID == "" {
		return nil, ErrInvalidArgument
	}
	if _, err := getManagedDevice(ctx, s.Devices, deviceID); err != nil {
		return nil, err
	}
	return mqttkit.CredentialForDevice(s.MQTT, deviceID, credentialID, deviceNow(s.Now), expiresAt), nil
}

func (s DeviceRuntimeAccessService) DeviceMQTTProfile(ctx context.Context, deviceID, credentialID string, expiresAt int64) (DeviceMQTTProfileView, error) {
	deviceID = normalizeDeviceID(deviceID)
	if deviceID == "" {
		return DeviceMQTTProfileView{}, ErrInvalidArgument
	}
	if _, err := getManagedDevice(ctx, s.Devices, deviceID); err != nil {
		return DeviceMQTTProfileView{}, err
	}
	networks, err := s.Networks.ListNetworksByDevice(ctx, deviceID)
	if err != nil {
		return DeviceMQTTProfileView{}, err
	}
	return newDeviceMQTTProfile(s.MQTT, deviceNow(s.Now), deviceID, credentialID, expiresAt, networks), nil
}

func (s DeviceCatalogService) GetDeviceProfile(ctx context.Context, deviceID string) (DeviceProfileView, error) {
	device, err := getManagedDevice(ctx, s.Devices, deviceID)
	if err != nil {
		return DeviceProfileView{}, err
	}
	return buildDeviceProfile(ctx, s.Networks, device)
}
