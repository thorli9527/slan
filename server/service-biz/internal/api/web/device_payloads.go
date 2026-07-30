package web

import servicepkg "github.com/slan/service-biz/internal/service"

func devicePayload(view servicepkg.DeviceProfileView) map[string]any {
	device := view.Device
	return map[string]any{
		"deviceId":       device.DeviceID,
		"ownerUserId":    device.OwnerID,
		"ownerId":        device.OwnerID,
		"name":           device.Name,
		"platform":       device.Platform,
		"osName":         device.OSName,
		"osVersion":      device.OSVersion,
		"publicKey":      device.PublicKey,
		"deviceVersion":  device.DeviceVersion,
		"countryCode":    device.CountryCode,
		"alias":          device.Alias,
		"status":         device.Status,
		"createdAt":      device.CreatedAt,
		"updatedAt":      device.UpdatedAt,
		"ownerEmail":     view.OwnerEmail,
		"globalIp":       view.GlobalIP,
		"globalName":     view.GlobalName,
		"networkEnabled": view.NetworkEnabled,
	}
}

func bootstrapKeyPayload(view servicepkg.DeviceBootstrapKeyView) map[string]any {
	return map[string]any{
		"id":                view.KeyID,
		"keyId":             view.KeyID,
		"installationKeyId": view.InstallationKeyID,
		"name":              view.Name,
		"key":               view.Token,
		"token":             view.Token,
		"installationKey":   view.InstallationKey,
		"createdByUserId":   view.UserID,
		"userId":            view.UserID,
		"networkId":         view.NetworkID,
		"deviceAlias":       view.DeviceAlias,
		"expiresAt":         view.ExpiresAt,
		"usedAt":            view.UsedAt,
		"usedByDeviceId":    view.UsedByDeviceID,
		"revokedAt":         view.RevokedAt,
		"status":            view.Status,
		"createdAt":         view.CreatedAt,
		"updatedAt":         view.UpdatedAt,
	}
}

func deviceLoginDevicePayload(view servicepkg.DeviceLoginDeviceView) map[string]any {
	return map[string]any{
		"deviceId":      view.DeviceID,
		"userId":        view.UserID,
		"name":          view.Name,
		"platform":      view.Platform,
		"alias":         view.Alias,
		"osName":        view.OSName,
		"osVersion":     view.OSVersion,
		"publicKey":     view.PublicKey,
		"deviceVersion": view.DeviceVersion,
		"countryCode":   view.CountryCode,
		"verifyCode":    view.VerifyCode,
		"status":        view.Status,
		"expiresAt":     view.ExpiresAt,
		"createdAt":     view.CreatedAt,
		"updatedAt":     view.UpdatedAt,
	}
}

func deviceLoginSummaryPayload(view servicepkg.DeviceLoginDeviceView) map[string]any {
	return map[string]any{
		"deviceId":      view.DeviceID,
		"status":        view.Status,
		"name":          view.Name,
		"platform":      view.Platform,
		"alias":         view.Alias,
		"osName":        view.OSName,
		"osVersion":     view.OSVersion,
		"publicKey":     view.PublicKey,
		"deviceVersion": view.DeviceVersion,
		"countryCode":   view.CountryCode,
	}
}

func preparedDeviceLoginPayload(view servicepkg.PrepareDeviceLoginDeviceView) map[string]any {
	payload := deviceLoginSummaryPayload(view.Device)
	payload["verifyCode"] = view.Device.VerifyCode
	payload["expiresAt"] = view.Device.ExpiresAt
	payload["device"] = deviceLoginDevicePayload(view.Device)
	payload["mqtt"] = view.Credential
	return payload
}

func completedDeviceLoginPayload(view servicepkg.CompleteDeviceLoginDeviceView) map[string]any {
	payload := deviceLoginSummaryPayload(view.Device)
	payload["device"] = deviceLoginDevicePayload(view.Device)
	return payload
}

func deviceGroupPayload(view servicepkg.DeviceGroupView) map[string]any {
	return map[string]any{
		"id":          view.GroupID,
		"groupId":     view.GroupID,
		"userOwnerId": view.UserID,
		"userId":      view.UserID,
		"name":        view.Name,
		"description": view.Description,
		"createdAt":   view.CreatedAt,
		"updatedAt":   view.UpdatedAt,
	}
}
