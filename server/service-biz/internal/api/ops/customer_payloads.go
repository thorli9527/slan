package ops

import (
	"strconv"

	servicepkg "github.com/slan/service-biz/internal/service"
)

func customerPayload(view servicepkg.OpsCustomerView) map[string]any {
	item := view.Customer
	return map[string]any{
		"customerId":     item.CustomerID,
		"email":          item.Email,
		"name":           item.Name,
		"country":        item.Country,
		"province":       item.Province,
		"city":           item.City,
		"ipRegion":       item.IPRegion,
		"planCode":       item.PlanCode,
		"planExpiresAt":  view.PlanExpiresAt,
		"ownDevices":     view.OwnDevices,
		"invitedDevices": view.InvitedDevices,
		"relayUsedGb":    view.RelayUsedGB,
		"status":         item.Status,
		"updatedAt":      item.UpdatedAt,
	}
}

func assignedCustomerPlanPayload(view servicepkg.OpsCustomerPlanAssignmentView) map[string]any {
	customer := view.Customer.Customer
	return map[string]any{
		"customer": customerPayload(view.Customer),
		"renewal": map[string]any{
			"renewalId":     "renewal-" + formatUnix(view.PaidAt),
			"customerId":    customer.CustomerID,
			"customerEmail": customer.Email,
			"planCode":      view.PlanCode,
			"period":        view.Period,
			"amount":        view.Amount,
			"paidAt":        view.PaidAt,
			"validUntil":    view.ValidUntil,
			"source":        "manual",
			"operator":      "ops",
		},
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

func formatUnix(value int64) string {
	return strconv.FormatInt(value, 10)
}
