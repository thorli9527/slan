package service

import "github.com/slan/service-biz/internal/model"

func applyRenewDeviceSessionInput(device model.Device, input RenewDeviceSessionInput, nowUnix int64) (model.Device, bool) {
	updated := false
	if input.RXBytesTotal > 0 && input.RXBytesTotal != device.RXBytesTotal {
		device.RXBytesTotal = input.RXBytesTotal
		updated = true
	}
	if input.TXBytesTotal > 0 && input.TXBytesTotal != device.TXBytesTotal {
		device.TXBytesTotal = input.TXBytesTotal
		updated = true
	}
	lastSeenAt := input.LastSeenAt
	if lastSeenAt <= 0 {
		lastSeenAt = nowUnix
	}
	if lastSeenAt != device.LastSeenAt {
		device.LastSeenAt = lastSeenAt
		updated = true
	}
	if updated {
		device.UpdatedAt = nowUnix
	}
	return device, updated
}
