package ops

import servicepkg "github.com/slan/service-biz/internal/service"

func userPayload(view servicepkg.OpsUserView) map[string]any {
	item := view.User
	return map[string]any{
		"userId":         item.UserID,
		"email":          item.Email,
		"name":           item.Name,
		"country":        item.Country,
		"province":       item.Province,
		"city":           item.City,
		"ipRegion":       item.IPRegion,
		"ownDevices":     view.OwnDevices,
		"invitedDevices": view.InvitedDevices,
		"relayUsedGb":    view.RelayUsedGB,
		"status":         item.Status,
		"updatedAt":      item.UpdatedAt,
	}
}

func managedDevicePayload(view servicepkg.OpsManagedDeviceView) map[string]any {
	item := view.Device
	return map[string]any{
		"deviceId":        item.DeviceID,
		"ownerId":         item.OwnerID,
		"ownerEmail":      view.OwnerEmail,
		"name":            item.Name,
		"alias":           item.Alias,
		"platform":        item.Platform,
		"osName":          item.OSName,
		"osVersion":       item.OSVersion,
		"globalIp":        view.GlobalIP,
		"globalName":      view.GlobalName,
		"status":          item.Status,
		"heartbeatOnline": view.HeartbeatOnline,
		"networkEnabled":  view.NetworkEnabled,
		"deviceEnabled":   view.DeviceEnabled,
		"rxBytesTotal":    item.RXBytesTotal,
		"txBytesTotal":    item.TXBytesTotal,
		"networkCount":    view.NetworkCount,
		"lastSeenAt":      item.LastSeenAt,
		"lastReportAt":    item.LastSeenAt,
		"createdAt":       item.CreatedAt,
		"updatedAt":       item.UpdatedAt,
	}
}
