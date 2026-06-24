package service

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

func (s DeviceCatalogService) ListDevices(ctx context.Context, ownerID string) ([]DeviceView, error) {
	items, err := listOwnedManagedDevices(ctx, s.Devices, ownerID)
	if err != nil {
		return nil, err
	}
	return deviceViews(items), nil
}

func (s DeviceCatalogService) ListVisibleDevices(ctx context.Context, ownerID string) ([]DeviceView, error) {
	return s.ListDevices(ctx, ownerID)
}

func (s DeviceCatalogService) GetDevice(ctx context.Context, deviceID string) (DeviceView, error) {
	item, err := getManagedDevice(ctx, s.Devices, deviceID)
	if err != nil {
		return DeviceView{}, err
	}
	return deviceView(item), nil
}

func (s DeviceProvisioningService) RegisterDevice(ctx context.Context, input RegisterDeviceInput) (DeviceProfileView, error) {
	item, err := registerManagedDevice(ctx, s.Users, s.Devices, s.Networks, s.Now, s.NewDeviceID, input)
	if err != nil {
		return DeviceProfileView{}, err
	}
	return buildDeviceProfile(ctx, s.Users, s.Networks, item)
}

func (s DeviceProvisioningService) UpdateDeviceAlias(ctx context.Context, input UpdateDeviceAliasInput) (DeviceProfileView, error) {
	device, err := s.updateDeviceAliasEntity(ctx, input)
	if err != nil {
		return DeviceProfileView{}, err
	}
	return buildDeviceProfile(ctx, s.Users, s.Networks, device)
}

func (s DeviceRuntimeAccessService) UpdateDeviceRuntime(ctx context.Context, input UpdateDeviceRuntimeInput) (DeviceProfileView, error) {
	device, err := s.updateDeviceRuntimeEntity(ctx, input)
	if err != nil {
		return DeviceProfileView{}, err
	}
	return buildDeviceProfile(ctx, s.Users, s.Networks, device)
}

func (s DeviceProvisioningService) updateDeviceAliasEntity(ctx context.Context, input UpdateDeviceAliasInput) (model.Device, error) {
	input = normalizeUpdateDeviceAliasInput(input)
	if input.DeviceID == "" {
		return model.Device{}, ErrInvalidArgument
	}
	device, err := requireOwnedManagedDevice(ctx, s.Users, s.Devices, input.ActorUserID, input.DeviceID)
	if err != nil {
		return model.Device{}, err
	}
	device = applyUpdateDeviceAlias(device, input.Alias, deviceNow(s.Now).Unix())
	if err := s.Devices.SaveDevice(ctx, device); err != nil {
		return model.Device{}, err
	}
	return device, nil
}

func (s DeviceRuntimeAccessService) updateDeviceRuntimeEntity(ctx context.Context, input UpdateDeviceRuntimeInput) (model.Device, error) {
	input = normalizeUpdateDeviceRuntimeInput(input)
	if input.DeviceID == "" {
		return model.Device{}, ErrInvalidArgument
	}
	now := deviceNow(s.Now).Unix()
	device, err := getManagedDevice(ctx, s.Devices, input.DeviceID)
	if err != nil {
		return model.Device{}, err
	}
	device = applyUpdateDeviceRuntime(device, input, now)
	if err := s.Devices.SaveDevice(ctx, device); err != nil {
		return model.Device{}, err
	}
	if err := s.updateDeviceRuntimeMembership(ctx, input, now); err != nil {
		return model.Device{}, err
	}
	return device, nil
}

func (s DeviceRuntimeAccessService) updateDeviceRuntimeMembership(ctx context.Context, input UpdateDeviceRuntimeInput, now int64) error {
	if !hasDeviceRuntimeMembershipUpdate(input) {
		return nil
	}
	networkID, membership, ok, err := resolveDeviceRuntimeMembership(ctx, s.Networks, input.NetworkID, input.DeviceID)
	if err != nil || !ok {
		return err
	}
	membership.NetworkID = networkID
	membership.DeviceID = input.DeviceID
	return s.Networks.SaveNetworkDevice(ctx, applyUpdateDeviceRuntimeMembership(membership, input, now))
}

func (s DeviceProvisioningService) DeleteDevice(ctx context.Context, input DeleteDeviceInput) error {
	input = normalizeDeleteDeviceInput(input)
	if input.DeviceID == "" {
		return ErrInvalidArgument
	}
	if _, err := requireOwnedManagedDevice(ctx, s.Users, s.Devices, input.ActorUserID, input.DeviceID); err != nil {
		return err
	}
	return s.Devices.DeleteDevice(ctx, input.DeviceID)
}

func (s DeviceProvisioningService) RenewDevice(ctx context.Context, deviceID string) (DeviceProfileView, error) {
	device, err := s.renewDeviceEntity(ctx, deviceID)
	if err != nil {
		return DeviceProfileView{}, err
	}
	return buildDeviceProfile(ctx, s.Users, s.Networks, device)
}

func (s DeviceProvisioningService) renewDeviceEntity(ctx context.Context, deviceID string) (model.Device, error) {
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

func (s DeviceRuntimeAccessService) DeviceMQTTCredential(ctx context.Context, deviceID string) (*mqttkit.Credential, error) {
	deviceID = normalizeDeviceID(deviceID)
	if deviceID == "" {
		return nil, ErrInvalidArgument
	}
	if _, err := getManagedDevice(ctx, s.Devices, deviceID); err != nil {
		return nil, err
	}
	return mqttkit.CredentialForDevice(s.MQTT, deviceID, deviceNow(s.Now)), nil
}

func (s DeviceRuntimeAccessService) DeviceMQTTProfile(ctx context.Context, deviceID string) (DeviceMQTTProfileView, error) {
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
	return newDeviceMQTTProfile(s.MQTT, deviceNow(s.Now), deviceID, networks), nil
}

func (s DeviceCatalogService) ListDeviceProfiles(ctx context.Context, ownerID string) ([]DeviceProfileView, error) {
	items, err := listOwnedManagedDevices(ctx, s.Devices, ownerID)
	if err != nil {
		return nil, err
	}
	return buildDeviceProfiles(ctx, s.Users, s.Networks, items)
}

func (s DeviceCatalogService) ListVisibleDeviceProfiles(ctx context.Context, ownerID string) ([]DeviceProfileView, error) {
	items, err := listOwnedManagedDevices(ctx, s.Devices, ownerID)
	if err != nil {
		return nil, err
	}
	return buildDeviceProfiles(ctx, s.Users, s.Networks, items)
}

func (s DeviceCatalogService) GetDeviceProfile(ctx context.Context, deviceID string) (DeviceProfileView, error) {
	device, err := getManagedDevice(ctx, s.Devices, deviceID)
	if err != nil {
		return DeviceProfileView{}, err
	}
	return buildDeviceProfile(ctx, s.Users, s.Networks, device)
}
