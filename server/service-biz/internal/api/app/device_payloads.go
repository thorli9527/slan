package app

import servicepkg "github.com/slan/service-biz/internal/service"

func appControlDevicePayload(deviceID, ownerID, name, platform, status string) map[string]any {
	return map[string]any{
		"deviceId":         deviceID,
		"activeNetworkId":  "",
		"ownerId":          ownerID,
		"ownerEmail":       "",
		"name":             name,
		"platform":         platform,
		"osName":           "",
		"osVersion":        "",
		"publicKey":        "",
		"deviceVersion":    "",
		"countryCode":      "",
		"rxBytesTotal":     int64(0),
		"txBytesTotal":     int64(0),
		"lastSeenAt":       int64(0),
		"status":           status,
		"membershipStatus": "",
		"currentVirtualIp": "",
		"virtualIp":        "",
		"globalIp":         "",
		"globalName":       "",
		"mqtt":             nil,
	}
}

func appControlDeviceProfilePayload(view servicepkg.DeviceProfileView) map[string]any {
	payload := appControlDevicePayload(
		view.Device.DeviceID,
		view.Device.OwnerID,
		view.Device.Name,
		view.Device.Platform,
		view.Device.Status,
	)
	payload["activeNetworkId"] = view.ActiveNetworkID
	payload["ownerEmail"] = view.OwnerEmail
	payload["membershipStatus"] = view.MembershipStatus
	payload["currentVirtualIp"] = view.CurrentVirtualIP
	payload["virtualIp"] = view.VirtualIP
	payload["globalIp"] = view.GlobalIP
	payload["globalName"] = view.GlobalName
	payload["osName"] = view.Device.OSName
	payload["osVersion"] = view.Device.OSVersion
	payload["publicKey"] = view.Device.PublicKey
	payload["deviceVersion"] = view.Device.DeviceVersion
	payload["countryCode"] = view.Device.CountryCode
	payload["rxBytesTotal"] = view.Device.RXBytesTotal
	payload["txBytesTotal"] = view.Device.TXBytesTotal
	payload["lastSeenAt"] = view.Device.LastSeenAt
	return payload
}

func renewedDevicePayload(view servicepkg.DeviceProfileView) map[string]any {
	return map[string]any{
		"device":         appControlDeviceProfilePayload(view),
		"networkConfigs": networkConfigsPayload([]map[string]any{}),
	}
}

func renewedDevicePayloadWithConfigs(view servicepkg.DeviceProfileView, networkConfigs []map[string]any) map[string]any {
	if len(networkConfigs) == 0 {
		return renewedDevicePayload(view)
	}
	return map[string]any{
		"device":         appControlDeviceProfilePayload(view),
		"networkConfigs": networkConfigsPayload(networkConfigs),
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
