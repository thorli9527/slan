package service

import (
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	tokenStatusActive  = "active"
	tokenStatusRevoked = "revoked"
	tokenStatusExpired = "expired"

	tokenModeShort  = "short"
	tokenModeLong   = "long"
	tokenModeManual = "manual"

	defaultDeviceAccessTTL        = 2 * time.Hour
	defaultDeviceRefreshShortTTL  = 30 * 24 * time.Hour
	defaultDeviceRefreshLongTTL   = 180 * 24 * time.Hour
	defaultDeviceRefreshManualTTL = 365 * 24 * time.Hour
	deviceRefreshRotationGrace    = 120 * time.Second
	defaultOperatorSessionTTL     = 24 * time.Hour
)

type tokenTTLSetting struct {
	name     string
	fallback time.Duration
	min      time.Duration
	max      time.Duration
}

var tokenTTLSettings = []tokenTTLSetting{
	{name: "SLAN_DEVICE_ACCESS_TOKEN_TTL", fallback: defaultDeviceAccessTTL, min: 5 * time.Minute, max: 24 * time.Hour},
	{name: "SLAN_DEVICE_REFRESH_SHORT_TTL", fallback: defaultDeviceRefreshShortTTL, min: 24 * time.Hour, max: 90 * 24 * time.Hour},
	{name: "SLAN_DEVICE_REFRESH_LONG_TTL", fallback: defaultDeviceRefreshLongTTL, min: 24 * time.Hour, max: 365 * 24 * time.Hour},
	{name: "SLAN_DEVICE_REFRESH_MANUAL_TTL", fallback: defaultDeviceRefreshManualTTL, min: 24 * time.Hour, max: 730 * 24 * time.Hour},
	{name: "SLAN_OPS_SESSION_TTL", fallback: defaultOperatorSessionTTL, min: 15 * time.Minute, max: 24 * time.Hour},
}

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

func deviceAccessTTL() (time.Duration, error) {
	return configuredTokenTTL(tokenTTLSettings[0])
}

func deviceRefreshTTL(mode string) (time.Duration, error) {
	switch normalizedSessionMode(mode) {
	case tokenModeLong:
		return configuredTokenTTL(tokenTTLSettings[2])
	case tokenModeManual:
		return configuredTokenTTL(tokenTTLSettings[3])
	default:
		return configuredTokenTTL(tokenTTLSettings[1])
	}
}

func operatorSessionTTL() (time.Duration, error) {
	return configuredTokenTTL(tokenTTLSettings[4])
}

func ValidateTokenTTLConfiguration() error {
	for _, setting := range tokenTTLSettings {
		if _, err := configuredTokenTTL(setting); err != nil {
			return err
		}
	}
	return nil
}

func configuredTokenTTL(setting tokenTTLSetting) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(setting.name))
	if raw == "" {
		return setting.fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value < setting.min || value > setting.max {
		return 0, fmt.Errorf("%s must be a duration between %s and %s", setting.name, setting.min, setting.max)
	}
	return value, nil
}
