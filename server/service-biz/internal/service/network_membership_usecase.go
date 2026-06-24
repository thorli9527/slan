package service

import "context"

func (s NetworkInviteService) ListNetworkDevices(ctx context.Context, networkID string) ([]NetworkDeviceView, error) {
	networkID = normalizeNetworkID(networkID)
	if networkID == "" {
		return []NetworkDeviceView{}, nil
	}
	if _, err := requireManagedNetwork(ctx, s.Networks, networkID); err != nil {
		return nil, err
	}
	items, err := s.Networks.ListNetworkDevices(ctx, networkID)
	if err != nil {
		return nil, err
	}
	return buildNetworkDeviceViews(ctx, s.Devices, items)
}

func (s NetworkInviteService) AddNetworkDevice(ctx context.Context, input AddNetworkDeviceInput) (NetworkDeviceView, error) {
	input = normalizeAddNetworkDeviceInput(input)
	if input.NetworkID == "" || input.DeviceID == "" {
		return NetworkDeviceView{}, ErrInvalidArgument
	}
	if _, err := requireOwnedManagedNetwork(ctx, s.Users, s.Networks, input.ActorUserID, input.NetworkID); err != nil {
		return NetworkDeviceView{}, err
	}
	if _, err := requireManagedDevice(ctx, s.Devices, input.DeviceID); err != nil {
		return NetworkDeviceView{}, err
	}
	if err := ensureNetworkDeviceAlias(ctx, s.Devices, s.Now, input.ActorUserID, input.DeviceID, input.Alias); err != nil {
		return NetworkDeviceView{}, err
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	now := networkNow(s.Now).Unix()
	item := newNetworkDeviceMembership(input.NetworkID, input.DeviceID, enabled, now)
	if err := s.Networks.SaveNetworkDevice(ctx, item); err != nil {
		return NetworkDeviceView{}, err
	}
	return buildNetworkDeviceView(ctx, s.Devices, item)
}

func (s NetworkInviteService) UpdateNetworkDevice(ctx context.Context, input UpdateNetworkDeviceInput) (NetworkDeviceView, error) {
	input = normalizeUpdateNetworkDeviceInput(input)
	if input.NetworkID == "" || input.DeviceID == "" {
		return NetworkDeviceView{}, ErrInvalidArgument
	}
	if _, err := requireOwnedManagedNetwork(ctx, s.Users, s.Networks, input.ActorUserID, input.NetworkID); err != nil {
		return NetworkDeviceView{}, err
	}
	if err := ensureNetworkDeviceAlias(ctx, s.Devices, s.Now, input.ActorUserID, input.DeviceID, input.Alias); err != nil {
		return NetworkDeviceView{}, err
	}
	items, err := s.Networks.ListNetworkDevices(ctx, input.NetworkID)
	if err != nil {
		return NetworkDeviceView{}, err
	}
	item, ok := findNetworkDeviceMembership(items, input.DeviceID)
	if !ok {
		return NetworkDeviceView{}, ErrNotFound
	}
	item = applyUpdateNetworkDeviceInput(item, input, networkNow(s.Now).Unix())
	if err := s.Networks.SaveNetworkDevice(ctx, item); err != nil {
		return NetworkDeviceView{}, err
	}
	return buildNetworkDeviceView(ctx, s.Devices, item)
}

func (s NetworkInviteService) RemoveNetworkDevice(ctx context.Context, input RemoveNetworkDeviceInput) error {
	input = normalizeRemoveNetworkDeviceInput(input)
	if input.NetworkID == "" || input.DeviceID == "" {
		return ErrInvalidArgument
	}
	if _, err := requireOwnedManagedNetwork(ctx, s.Users, s.Networks, input.ActorUserID, input.NetworkID); err != nil {
		return err
	}
	return s.Networks.DeleteNetworkDevice(ctx, input.NetworkID, input.DeviceID)
}
