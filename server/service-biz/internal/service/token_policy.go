package service

import "time"

const (
	tokenStatusActive  = "active"
	tokenStatusRevoked = "revoked"
	tokenStatusExpired = "expired"

	tokenModeShort  = "short"
	tokenModeLong   = "long"
	tokenModeManual = "manual"

	installationKeyStatusUsed = "used"
)

const (
	defaultUserAccessTTL      = 2 * time.Hour
	defaultUserRefreshShortTTL = 7 * 24 * time.Hour
	defaultUserRefreshLongTTL  = 90 * 24 * time.Hour
	defaultUserRefreshManualTTL = 180 * 24 * time.Hour

	defaultDeviceAccessTTL      = 2 * time.Hour
	defaultDeviceRefreshShortTTL = 30 * 24 * time.Hour
	defaultDeviceRefreshLongTTL  = 180 * 24 * time.Hour
	defaultDeviceRefreshManualTTL = 365 * 24 * time.Hour

	defaultInstallationKeyTTL = 30 * time.Minute
	maxInstallationKeyTTL     = 1 * time.Hour
)

func normalizedSessionMode(mode string) string {
	switch mode {
	case tokenModeLong:
		return tokenModeLong
	case tokenModeManual:
		return tokenModeManual
	default:
		return tokenModeShort
	}
}

func userRefreshTTL(mode string) time.Duration {
	switch normalizedSessionMode(mode) {
	case tokenModeLong:
		return defaultUserRefreshLongTTL
	case tokenModeManual:
		return defaultUserRefreshManualTTL
	default:
		return defaultUserRefreshShortTTL
	}
}

func deviceRefreshTTL(mode string) time.Duration {
	switch normalizedSessionMode(mode) {
	case tokenModeLong:
		return defaultDeviceRefreshLongTTL
	case tokenModeManual:
		return defaultDeviceRefreshManualTTL
	default:
		return defaultDeviceRefreshShortTTL
	}
}

func installationKeyTTL(ttlSeconds int64) time.Duration {
	ttl := defaultInstallationKeyTTL
	if ttlSeconds > 0 {
		ttl = time.Duration(ttlSeconds) * time.Second
	}
	if ttl > maxInstallationKeyTTL {
		ttl = maxInstallationKeyTTL
	}
	return ttl
}
