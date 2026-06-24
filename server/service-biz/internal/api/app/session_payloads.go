package app

import servicepkg "github.com/slan/service-biz/internal/service"

func appControlDeviceSessionPayload(profile servicepkg.DeviceProfileView, sessionID, deviceID, accessToken string, expiresAt int64, refreshToken string, activeNetworkIDs []string) map[string]any {
	return map[string]any{
		"sessionId":            sessionID,
		"deviceId":             deviceID,
		"userId":               profile.Device.OwnerID,
		"deviceToken":          accessToken,
		"deviceTokenExpiresAt": expiresAt,
		"deviceRefreshToken":   refreshToken,
		"activeNetworkIds":     sessionActiveNetworkIDs(profile, activeNetworkIDs),
	}
}

func sessionActiveNetworkIDs(profile servicepkg.DeviceProfileView, activeNetworkIDs []string) []string {
	values := make([]string, 0, len(activeNetworkIDs)+1)
	if profile.ActiveNetworkID != "" {
		values = append(values, profile.ActiveNetworkID)
	}
	values = append(values, activeNetworkIDs...)
	return uniqueStrings(values)
}

func deviceSessionPayload(view servicepkg.DeviceSessionBootstrapView, punchNodes []servicepkg.PunchNodeView, networkConfigs []map[string]any) map[string]any {
	return deviceSessionEnvelope(
		view.Profile,
		view.Session.SessionID,
		view.Session.DeviceID,
		view.Session.AccessToken,
		view.Session.ExpiresAt,
		view.Session.RefreshToken,
		view.Session.UpdatedAt,
		view.MQTT,
		punchNodes,
		networkConfigs,
	)
}

func boundDeviceSessionPayload(view servicepkg.DeviceSessionBoundView, punchNodes []servicepkg.PunchNodeView, networkConfigs []map[string]any) map[string]any {
	return deviceSessionEnvelope(
		view.Profile,
		view.Session.SessionID,
		view.Session.DeviceID,
		view.Session.AccessToken,
		view.Session.ExpiresAt,
		view.Session.RefreshToken,
		view.Session.UpdatedAt,
		view.MQTT,
		punchNodes,
		networkConfigs,
	)
}

func deviceSessionEnvelope(
	profile servicepkg.DeviceProfileView,
	sessionID string,
	deviceID string,
	accessToken string,
	expiresAt int64,
	refreshToken string,
	updatedAt int64,
	mqtt servicepkg.DeviceMQTTProfileView,
	punchNodes []servicepkg.PunchNodeView,
	networkConfigs []map[string]any,
) map[string]any {
	mqttPayload := appMQTTCredentialPayload(mqtt)
	device := appControlDeviceProfilePayload(profile)
	device["mqtt"] = mqttPayload
	return map[string]any{
		"device":         device,
		"deviceSession":  appControlDeviceSessionPayload(profile, sessionID, deviceID, accessToken, expiresAt, refreshToken, mqtt.NetworkIDs),
		"mqtt":           mqttPayload,
		"networkConfigs": networkConfigsPayload(networkConfigs),
		"runtimeEndpoints": runtimeEndpointsPayload(
			mqtt,
			punchNodes,
			networkConfigs,
			updatedAt,
		),
	}
}
