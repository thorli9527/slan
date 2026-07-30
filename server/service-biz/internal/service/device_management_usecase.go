package service

import (
	"context"
	"time"

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
	items, err := listVisibleManagedDevices(ctx, s.Devices, s.Relations, ownerID)
	if err != nil {
		return nil, err
	}
	views := deviceViews(items)
	aliases, err := s.visibleDeviceAliases(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	for i := range views {
		if alias := aliases[views[i].DeviceID]; alias != "" {
			views[i].Alias = alias
		}
	}
	return views, nil
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
	device, alias, err := s.updateDeviceAliasEntity(ctx, input)
	if err != nil {
		return DeviceProfileView{}, err
	}
	view, err := buildDeviceProfile(ctx, s.Users, s.Networks, device)
	if err != nil {
		return DeviceProfileView{}, err
	}
	if alias != "" {
		view.Device.Alias = alias
		view.GlobalName = networkGlobalName(device.DeviceID, alias, device.Name)
	}
	return view, nil
}

func (s DeviceRuntimeAccessService) UpdateDeviceRuntime(ctx context.Context, input UpdateDeviceRuntimeInput) (DeviceProfileView, error) {
	device, err := s.updateDeviceRuntimeEntity(ctx, input)
	if err != nil {
		return DeviceProfileView{}, err
	}
	return buildDeviceProfile(ctx, s.Users, s.Networks, device)
}

func (s DeviceProvisioningService) updateDeviceAliasEntity(ctx context.Context, input UpdateDeviceAliasInput) (model.Device, string, error) {
	input = normalizeUpdateDeviceAliasInput(input)
	if input.DeviceID == "" || input.ActorUserID == "" {
		return model.Device{}, "", ErrInvalidArgument
	}
	device, err := getManagedDevice(ctx, s.Devices, input.DeviceID)
	if err != nil {
		return model.Device{}, "", err
	}
	relation, ok, err := s.Relations.GetDeviceUserRelation(ctx, input.DeviceID, input.ActorUserID)
	if err != nil {
		return model.Device{}, "", err
	}
	if !ok || relation.Status != model.DeviceRelationStatusActive {
		return model.Device{}, "", ErrForbidden
	}
	if relation.Role == model.DeviceRelationRoleShared {
		relation.Alias = input.Alias
		relation.UpdatedAt = deviceNow(s.Now).Unix()
		if err := s.Relations.SaveDeviceUserRelation(ctx, relation); err != nil {
			return model.Device{}, "", err
		}
		return device, relation.Alias, nil
	}
	if relation.Role != model.DeviceRelationRoleOwner || device.OwnerID != input.ActorUserID {
		return model.Device{}, "", ErrForbidden
	}
	device = applyUpdateDeviceAlias(device, input.Alias, deviceNow(s.Now).Unix())
	if err := s.Devices.SaveDevice(ctx, device); err != nil {
		return model.Device{}, "", err
	}
	return device, device.Alias, nil
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

func (s DeviceProvisioningService) DeleteDevice(ctx context.Context, input DeleteDeviceInput) error {
	input = normalizeDeleteDeviceInput(input)
	if input.DeviceID == "" || input.ActorUserID == "" {
		return ErrInvalidArgument
	}
	device, err := requireManagedDevice(ctx, s.Devices, input.DeviceID)
	if err != nil {
		return err
	}
	if device.OwnerID != input.ActorUserID {
		return ErrForbidden
	}
	return ErrForbidden
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
	items, err := listVisibleManagedDevices(ctx, s.Devices, s.Relations, ownerID)
	if err != nil {
		return nil, err
	}
	views, err := buildDeviceProfiles(ctx, s.Users, s.Networks, items)
	if err != nil {
		return nil, err
	}
	aliases, err := s.visibleDeviceAliases(ctx, ownerID)
	if err != nil {
		return nil, err
	}
	for i := range views {
		if alias := aliases[views[i].Device.DeviceID]; alias != "" {
			views[i].Device.Alias = alias
			views[i].GlobalName = networkGlobalName(views[i].Device.DeviceID, alias, views[i].Device.Name)
		}
	}
	return views, nil
}

func (s DeviceCatalogService) visibleDeviceAliases(ctx context.Context, userID string) (map[string]string, error) {
	relations, err := s.Relations.ListDeviceRelationsByUser(ctx, normalizeDeviceOwnerID(userID))
	if err != nil {
		return nil, err
	}
	aliases := make(map[string]string, len(relations))
	for _, relation := range relations {
		if relation.Status == model.DeviceRelationStatusActive && relation.Alias != "" {
			aliases[relation.DeviceID] = relation.Alias
		}
	}
	return aliases, nil
}

func (s DeviceCatalogService) GetDeviceProfile(ctx context.Context, deviceID string) (DeviceProfileView, error) {
	device, err := getManagedDevice(ctx, s.Devices, deviceID)
	if err != nil {
		return DeviceProfileView{}, err
	}
	return buildDeviceProfile(ctx, s.Users, s.Networks, device)
}
