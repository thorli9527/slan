package ops

import servicepkg "github.com/slan/service-biz/internal/service"

func clientDownloadPayload(view servicepkg.ClientDownloadView) map[string]any {
	return map[string]any{
		"downloadId":   view.DownloadID,
		"name":         view.Name,
		"platform":     view.Platform,
		"platformName": view.PlatformName,
		"version":      view.Version,
		"arch":         view.Arch,
		"channel":      view.Channel,
		"fileName":     view.Name,
		"fileSize":     view.FileSize,
		"url":          view.URL,
		"downloadUrl":  view.URL,
		"sha256":       view.SHA256,
		"releaseNotes": view.ReleaseNotes,
		"status":       view.Status,
		"createdAt":    view.CreatedAt,
		"updatedAt":    view.UpdatedAt,
	}
}

func planPayload(view servicepkg.PlanView) map[string]any {
	return map[string]any{
		"code":               view.PlanCode,
		"planCode":           view.PlanCode,
		"name":               view.Name,
		"ownDeviceLimit":     view.DeviceLimit,
		"invitedDeviceLimit": view.InvitedDeviceLimit,
		"totalDeviceLimit":   view.TotalDeviceLimit,
		"relayMonthlyGb":     view.RelayMonthlyGB,
		"relayBandwidthMbps": view.RelayBandwidthMbps,
		"relayThrottleMbps":  view.RelayThrottleMbps,
		"p2pUnlimited":       view.P2PUnlimited,
		"customDomain":       view.CustomDomain,
		"acl":                view.ACL,
		"dedicatedRelay":     view.DedicatedRelay,
		"auditLog":           view.AuditLog,
		"apiAccess":          view.APIAccess,
		"monthlyPrice":       view.MonthlyPrice,
		"yearlyPrice":        view.YearlyPrice,
		"status":             view.Status,
		"updatedAt":          view.UpdatedAt,
	}
}

func productPayload(view servicepkg.ProductView) map[string]any {
	return map[string]any{
		"productId":          view.ProductID,
		"name":               view.Name,
		"type":               view.Type,
		"planCode":           view.PlanCode,
		"period":             view.Period,
		"validDays":          view.ValidDays,
		"relayTrafficGb":     view.RelayTrafficGB,
		"relayBandwidthMbps": view.RelayBandwidthMbps,
		"listPrice":          view.Price,
		"salePrice":          view.SalePrice,
		"currency":           view.Currency,
		"autoRenew":          view.AutoRenew,
		"status":             view.Status,
		"description":        view.Description,
		"createdAt":          view.CreatedAt,
		"updatedAt":          view.UpdatedAt,
	}
}

func orderPayload(view servicepkg.OrderView) map[string]any {
	return map[string]any{
		"orderId":         view.OrderID,
		"customerId":      view.CustomerID,
		"customerEmail":   view.CustomerEmail,
		"productId":       view.ProductID,
		"productName":     view.ProductName,
		"productType":     view.ProductType,
		"amount":          view.Amount,
		"currency":        view.Currency,
		"payStatus":       view.PayStatus,
		"provisionStatus": view.ProvisionStatus,
		"createdAt":       view.CreatedAt,
		"updatedAt":       view.UpdatedAt,
		"channel":         view.Channel,
		"paidAt":          view.PaidAt,
		"validUntil":      view.ValidUntil,
	}
}

func renewalPayload(view servicepkg.RenewalView) map[string]any {
	return map[string]any{
		"renewalId":     view.RenewalID,
		"orderId":       view.OrderID,
		"customerEmail": view.CustomerEmail,
		"planCode":      view.PlanCode,
		"period":        view.Period,
		"amount":        view.Amount,
		"status":        view.Status,
		"paidAt":        view.PaidAt,
		"validUntil":    view.RenewAt,
		"updatedAt":     view.UpdatedAt,
		"source":        view.Source,
		"operator":      view.Operator,
	}
}
