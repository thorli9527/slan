package service

import "github.com/slan/service-biz/internal/model"

func applyBindDeviceSessionInput(device model.Device, input BindDeviceSessionInput, nowUnix int64) (model.Device, bool) {
	updated := false
	if input.Name != "" && input.Name != device.Name {
		device.Name = input.Name
		updated = true
	}
	if input.Platform != "" && input.Platform != device.Platform {
		device.Platform = input.Platform
		updated = true
	}
	if input.Alias != "" && input.Alias != device.Alias {
		device.Alias = input.Alias
		updated = true
	}
	if input.OSName != "" && input.OSName != device.OSName {
		device.OSName = input.OSName
		updated = true
	}
	if input.OSVersion != "" && input.OSVersion != device.OSVersion {
		device.OSVersion = input.OSVersion
		updated = true
	}
	if input.PublicKey != "" && input.PublicKey != device.PublicKey {
		device.PublicKey = input.PublicKey
		updated = true
	}
	if input.DeviceVersion != "" && input.DeviceVersion != device.DeviceVersion {
		device.DeviceVersion = input.DeviceVersion
		updated = true
	}
	if input.CountryCode != "" && input.CountryCode != device.CountryCode {
		device.CountryCode = input.CountryCode
		updated = true
	}
	if input.RXBytesTotal > 0 && input.RXBytesTotal != device.RXBytesTotal {
		device.RXBytesTotal = input.RXBytesTotal
		updated = true
	}
	if input.TXBytesTotal > 0 && input.TXBytesTotal != device.TXBytesTotal {
		device.TXBytesTotal = input.TXBytesTotal
		updated = true
	}
	if input.LastSeenAt > 0 && input.LastSeenAt != device.LastSeenAt {
		device.LastSeenAt = input.LastSeenAt
		updated = true
	}
	if input.LastSeenAt == 0 && nowUnix != device.LastSeenAt {
		device.LastSeenAt = nowUnix
		updated = true
	}
	if updated {
		device.UpdatedAt = nowUnix
	}
	return device, updated
}

func applyRenewDeviceSessionInput(device model.Device, input RenewDeviceSessionInput, nowUnix int64) (model.Device, bool) {
	updated := false
	if input.NetworkEnabled != nil {
		status := "inactive"
		if *input.NetworkEnabled {
			status = "active"
		}
		if device.Status != status {
			device.Status = status
			updated = true
		}
	}
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
