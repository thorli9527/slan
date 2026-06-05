package biz

import "strings"

const (
	deviceIDMaxLength        = 128
	deviceNameMaxLength      = 128
	devicePlatformMaxLength  = 32
	deviceOSNameMaxLength    = 64
	deviceOSVersionMaxLength = 64
	deviceAliasMaxLength     = 128
)

func normalizeDeviceIdentity(req DeviceIdentityRequest) (DeviceIdentityRequest, error) {
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	if req.DeviceID == "" || len(req.DeviceID) > deviceIDMaxLength {
		return DeviceIdentityRequest{}, errBadRequest
	}
	req.Name = trimToMax(req.Name, deviceNameMaxLength)
	req.Platform = trimToMax(req.Platform, devicePlatformMaxLength)
	req.OSName = trimToMax(req.OSName, deviceOSNameMaxLength)
	req.OSVersion = trimToMax(req.OSVersion, deviceOSVersionMaxLength)
	req.Alias = trimToMax(req.Alias, deviceAliasMaxLength)
	req.PublicKey = strings.TrimSpace(req.PublicKey)
	req.DeviceVersion = strings.TrimSpace(req.DeviceVersion)
	return req, nil
}

func trimToMax(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max]
}
