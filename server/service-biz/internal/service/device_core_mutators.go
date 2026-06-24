package service

import "github.com/slan/service-biz/internal/model"

func applyUpdateDeviceAlias(device model.Device, alias string, now int64) model.Device {
	device.Alias = alias
	device.UpdatedAt = now
	return device
}

func applyUpdateDeviceRuntime(device model.Device, input UpdateDeviceRuntimeInput, now int64) model.Device {
	if input.Platform != "" {
		device.Platform = input.Platform
	}
	if input.RXBytesTotal > 0 {
		device.RXBytesTotal = input.RXBytesTotal
	}
	if input.TXBytesTotal > 0 {
		device.TXBytesTotal = input.TXBytesTotal
	}
	if input.LastSeenAt > 0 {
		device.LastSeenAt = input.LastSeenAt
	} else if input.ReportedAtMS > 0 {
		device.LastSeenAt = input.ReportedAtMS / 1000
	} else {
		device.LastSeenAt = now
	}
	if input.Status != "" {
		device.Status = input.Status
	} else if device.Status == "" {
		device.Status = "active"
	}
	if input.DeviceVersion != "" {
		device.DeviceVersion = input.DeviceVersion
	}
	device.UpdatedAt = now
	return device
}

func renewManagedDevice(device model.Device, now int64) model.Device {
	device.Status = "active"
	device.LastSeenAt = now
	device.UpdatedAt = now
	return device
}

func applyUpdateDeviceRuntimeMembership(item model.NetworkDevice, input UpdateDeviceRuntimeInput, now int64) model.NetworkDevice {
	if input.NATType != "" {
		item.NATType = input.NATType
	}
	if input.ActivePath != "" {
		item.ActivePath = input.ActivePath
	}
	if input.PathObservedAt > 0 {
		item.PathObservedAt = input.PathObservedAt
	} else if input.ReportedAtMS > 0 {
		item.PathObservedAt = input.ReportedAtMS
	} else if item.PathObservedAt <= 0 {
		item.PathObservedAt = now * 1000
	}
	if input.RelayTransport != "" {
		item.RelayTransport = input.RelayTransport
	}
	if input.RelayEndpoint != "" {
		item.RelayEndpoint = input.RelayEndpoint
	}
	if input.DerpNodeID != "" {
		item.DerpNodeID = input.DerpNodeID
	}
	if input.PeerNodeID != "" {
		item.PeerNodeID = input.PeerNodeID
	}
	if input.PathScore > 0 {
		item.PathScore = input.PathScore
	}
	if input.ObservedRttMs > 0 {
		item.ObservedRttMs = input.ObservedRttMs
	}
	if input.PacketLossPpm > 0 {
		item.PacketLossPpm = input.PacketLossPpm
	}
	if input.RelayMtu > 0 {
		item.RelayMtu = input.RelayMtu
	}
	if input.MaxFramePayload > 0 {
		item.MaxFramePayload = input.MaxFramePayload
	}
	if input.TicketExpiresAt != "" {
		item.TicketExpiresAt = input.TicketExpiresAt
	}
	if input.TicketRenewDue != nil {
		item.TicketRenewDue = *input.TicketRenewDue
	}
	if input.PathDowngrades > 0 {
		item.PathDowngrades = input.PathDowngrades
	}
	if input.PathUpgrades > 0 {
		item.PathUpgrades = input.PathUpgrades
	}
	if input.LastPathChange != "" {
		item.LastPathChange = input.LastPathChange
	}
	item.UpdatedAt = now
	return item
}
