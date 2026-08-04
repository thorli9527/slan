package service

import (
	"testing"
	"time"
)

func TestTokenTTLConfigurationUsesDefaults(t *testing.T) {
	clearTokenTTLSettings(t)
	access, err := deviceAccessTTL()
	if err != nil || access != defaultDeviceAccessTTL {
		t.Fatalf("default device access TTL = %s err=%v", access, err)
	}
	operator, err := operatorSessionTTL()
	if err != nil || operator != defaultOperatorSessionTTL {
		t.Fatalf("default operator session TTL = %s err=%v", operator, err)
	}
}

func TestTokenTTLConfigurationUsesExplicitDurations(t *testing.T) {
	clearTokenTTLSettings(t)
	t.Setenv("SLAN_DEVICE_ACCESS_TOKEN_TTL", "45m")
	t.Setenv("SLAN_DEVICE_REFRESH_LONG_TTL", "2400h")
	t.Setenv("SLAN_OPS_SESSION_TTL", "8h")
	access, _ := deviceAccessTTL()
	refresh, _ := deviceRefreshTTL(tokenModeLong)
	operator, _ := operatorSessionTTL()
	if access != 45*time.Minute || refresh != 100*24*time.Hour || operator != 8*time.Hour {
		t.Fatalf("configured TTLs = access %s refresh %s operator %s", access, refresh, operator)
	}
}

func TestTokenTTLConfigurationRejectsInvalidOrUnsafeDurations(t *testing.T) {
	for _, value := range []string{"invalid", "1m", "25h"} {
		t.Run(value, func(t *testing.T) {
			clearTokenTTLSettings(t)
			t.Setenv("SLAN_DEVICE_ACCESS_TOKEN_TTL", value)
			if err := ValidateTokenTTLConfiguration(); err == nil {
				t.Fatalf("device access TTL %q was accepted", value)
			}
		})
	}
}

func TestConfiguredTTLsApplyToIssuedSessions(t *testing.T) {
	clearTokenTTLSettings(t)
	t.Setenv("SLAN_DEVICE_ACCESS_TOKEN_TTL", "45m")
	t.Setenv("SLAN_DEVICE_REFRESH_LONG_TTL", "2400h")
	t.Setenv("SLAN_OPS_SESSION_TTL", "8h")
	now := time.Unix(1_700_000_000, 0)

	device, err := newManagedDeviceSession(now, func(scope string) string { return scope + "-1" }, "device-1", tokenModeLong)
	if err != nil {
		t.Fatal(err)
	}
	operator, err := newOperatorSession(now, func(scope string) string { return scope + "-1" }, "operator-1")
	if err != nil {
		t.Fatal(err)
	}
	if device.ExpiresAt != now.Add(45*time.Minute).Unix() || device.RefreshExpiry != now.Add(100*24*time.Hour).Unix() {
		t.Fatalf("configured device session expiry = %+v", device)
	}
	if operator.ExpiresAt != now.Add(8*time.Hour).Unix() {
		t.Fatalf("configured operator session expiry = %d", operator.ExpiresAt)
	}
}

func clearTokenTTLSettings(t *testing.T) {
	t.Helper()
	for _, setting := range tokenTTLSettings {
		t.Setenv(setting.name, "")
	}
}
