package app

import servicepkg "github.com/slan/service-biz/internal/service"

func appControlDeviceSessionPayload(profile servicepkg.DeviceProfileView, sessionID, deviceID, accessToken string, expiresAt int64, refreshToken string, activeNetworkIDs []string, session servicepkg.DeviceSessionView) map[string]any {
	return map[string]any{
		"sessionId":            sessionID,
		"deviceId":             deviceID,
		"deviceToken":          accessToken,
		"deviceTokenExpiresAt": expiresAt,
		"deviceRefreshToken":   refreshToken,
		"deviceRefreshExpiry":  session.RefreshExpiry,
		"sessionMode":          session.SessionMode,
		"status":               session.Status,
		"revokedAt":            session.RevokedAt,
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

func boundDeviceSessionPayload(view servicepkg.DeviceSessionBoundView, punchNodes []servicepkg.PunchNodeView, proxyNodes []servicepkg.OpsServerNodeView, networkConfigs []map[string]any) map[string]any {
	return deviceSessionEnvelope(
		view.Profile,
		view.Session,
		view.MQTT,
		punchNodes,
		proxyNodes,
		networkConfigs,
	)
}

func deviceSessionEnvelope(
	profile servicepkg.DeviceProfileView,
	session servicepkg.DeviceSessionView,
	mqtt servicepkg.DeviceMQTTProfileView,
	punchNodes []servicepkg.PunchNodeView,
	proxyNodes []servicepkg.OpsServerNodeView,
	networkConfigs []map[string]any,
) map[string]any {
	mqttPayload := appMQTTCredentialPayload(mqtt)
	device := appControlDeviceProfilePayload(profile)
	device["mqtt"] = mqttPayload
	return map[string]any{
		"device":         device,
		"deviceSession":  appControlDeviceSessionPayload(profile, session.SessionID, session.DeviceID, session.AccessToken, session.ExpiresAt, session.RefreshToken, mqtt.NetworkIDs, session),
		"mqtt":           mqttPayload,
		"networkConfigs": networkConfigsPayload(networkConfigs),
		"runtimeEndpoints": runtimeEndpointsPayload(
			mqtt,
			punchNodes,
			proxyNodes,
			networkConfigs,
			session.UpdatedAt,
		),
	}
}
