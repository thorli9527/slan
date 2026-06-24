package service

import (
	"context"
)

func (s OpsManagedDeviceService) ListDevices(ctx context.Context) ([]OpsManagedDeviceView, error) {
	users, err := s.Users.ListUsers(ctx)
	if err != nil {
		return nil, err
	}
	return buildManagedDeviceViews(ctx, s.Devices, s.Networks, users)
}

func (s OpsManagedDeviceService) UpdateDevice(ctx context.Context, input UpdateDeviceInput) (OpsManagedDeviceView, error) {
	input = normalizeUpdateDeviceInput(input)
	if input.DeviceID == "" {
		return OpsManagedDeviceView{}, ErrInvalidArgument
	}
	item, err := requireOpsDevice(ctx, s.Devices, input.DeviceID)
	if err != nil {
		return OpsManagedDeviceView{}, err
	}
	item = applyUpdateDeviceInput(item, input, opsNow(s.Now).Unix())
	if err := s.Devices.SaveDevice(ctx, item); err != nil {
		return OpsManagedDeviceView{}, err
	}
	return buildManagedDeviceView(ctx, s.Users, s.Networks, item)
}

func (s OpsManagedDeviceService) DeleteDevice(ctx context.Context, deviceID string) error {
	return s.Devices.DeleteDevice(ctx, normalizeManagedDeviceID(deviceID))
}
