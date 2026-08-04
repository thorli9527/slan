package ops

import (
	"strconv"

	servicepkg "github.com/slan/service-biz/internal/service"
)

func customerPayload(view servicepkg.OpsCustomerView) map[string]any {
	item := view.Customer
	return map[string]any{
		"customerId":  item.CustomerID,
		"email":       item.Email,
		"name":        item.Name,
		"country":     item.Country,
		"province":    item.Province,
		"city":        item.City,
		"ipRegion":    item.IPRegion,
		"relayUsedGb": view.RelayUsedGB,
		"status":      item.Status,
		"updatedAt":   item.UpdatedAt,
	}
}

func managedDevicePayload(view servicepkg.OpsManagedDeviceView) map[string]any {
	item := view.Device
	return map[string]any{
		"deviceId":        item.DeviceID,
		"name":            item.Name,
		"alias":           item.Alias,
		"platform":        item.Platform,
		"osName":          item.OSName,
		"osVersion":       item.OSVersion,
		"deviceVersion":   item.DeviceVersion,
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

func formatUnix(value int64) string {
	return strconv.FormatInt(value, 10)
}
