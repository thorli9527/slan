package service

import "github.com/slan/service-biz/internal/model"

func applyUpdateDeviceInput(device model.Device, input UpdateDeviceInput, now int64) model.Device {
	if input.Name != "" {
		device.Name = input.Name
	}
	if input.Alias != "" {
		device.Alias = input.Alias
	}
	if input.Status != "" {
		device.Status = input.Status
	}
	device.UpdatedAt = now
	return device
}
