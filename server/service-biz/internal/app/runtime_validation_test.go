package app

import (
	"strings"
	"testing"
)

func TestValidateRuntimeEnvironmentAllowsDevelopmentDefaultsOutsideProduction(t *testing.T) {
	t.Setenv("SLAN_ENV", "development")
	t.Setenv("SLAN_DEVICE_CREDENTIAL_PEPPER", "")
	if err := ValidateRuntimeEnvironment(); err != nil {
		t.Fatalf("development runtime validation: %v", err)
	}
}

func TestValidateRuntimeEnvironmentRejectsMissingProductionSecrets(t *testing.T) {
	t.Setenv("SLAN_ENV", "production")
	t.Setenv("SLAN_DEVICE_CREDENTIAL_PEPPER", "")
	err := ValidateRuntimeEnvironment()
	if err == nil || !strings.Contains(err.Error(), "SLAN_DEVICE_CREDENTIAL_PEPPER") {
		t.Fatalf("missing pepper error = %v", err)
	}
}

func TestValidateRuntimeEnvironmentAcceptsExplicitProductionSecrets(t *testing.T) {
	t.Setenv("SLAN_ENV", "prod")
	t.Setenv("SLAN_DEVICE_CREDENTIAL_PEPPER", "production-device-pepper-0123456789abcdef")
	t.Setenv("SLAN_MQTT_PASSWORD_SECRET", "production-mqtt-secret-0123456789abcdef")
	t.Setenv("SLAN_INTERNAL_WIRE_TOKEN", "production-wire-token-0123456789abcdef")
	t.Setenv("SLAN_MQTT_WEBHOOK_TOKEN", "production-mqtt-webhook-0123456789abcdef")
	t.Setenv("SLAN_SERVICE_BIZ_DB_PASSWORD", "production-db-password-0123456789")
	t.Setenv("SLAN_MQTT_PUBLIC_BROKER_URL", "mqtts://mqtt.example.com:8883")
	if err := ValidateRuntimeEnvironment(); err != nil {
		t.Fatalf("production runtime validation: %v", err)
	}
}

func TestValidateRuntimeEnvironmentRejectsDefaultDatabasePassword(t *testing.T) {
	setValidProductionEnvironment(t)
	t.Setenv("SLAN_SERVICE_BIZ_DB_PASSWORD", "slan")
	err := ValidateRuntimeEnvironment()
	if err == nil || !strings.Contains(err.Error(), "PostgreSQL password") {
		t.Fatalf("default database password error = %v", err)
	}
}

func TestValidateRuntimeEnvironmentRejectsInsecureMQTTEndpoint(t *testing.T) {
	setValidProductionEnvironment(t)
	t.Setenv("SLAN_MQTT_PUBLIC_BROKER_URL", "mqtt://127.0.0.1:1883")
	err := ValidateRuntimeEnvironment()
	if err == nil || !strings.Contains(err.Error(), "SLAN_MQTT_PUBLIC_BROKER_URL") {
		t.Fatalf("insecure MQTT endpoint error = %v", err)
	}
}

func TestValidateRuntimeEnvironmentReadsPasswordFromDSN(t *testing.T) {
	setValidProductionEnvironment(t)
	t.Setenv("SLAN_SERVICE_BIZ_DSN", "postgres://slan:dsn-production-password-012345@postgres:5432/slan?sslmode=require")
	if err := ValidateRuntimeEnvironment(); err != nil {
		t.Fatalf("production DSN validation: %v", err)
	}
}

func TestValidateRuntimeEnvironmentRejectsWeakPreviousDevicePepper(t *testing.T) {
	setValidProductionEnvironment(t)
	t.Setenv("SLAN_DEVICE_CREDENTIAL_PREVIOUS_PEPPERS", "weak-old-pepper")
	err := ValidateRuntimeEnvironment()
	if err == nil || !strings.Contains(err.Error(), "SLAN_DEVICE_CREDENTIAL_PREVIOUS_PEPPERS") {
		t.Fatalf("weak previous pepper error = %v", err)
	}
}

func TestValidateRuntimeEnvironmentRejectsInvalidTrustedProxyCIDR(t *testing.T) {
	setValidProductionEnvironment(t)
	t.Setenv("SLAN_TRUSTED_PROXY_CIDRS", "10.0.0.0/8,invalid")
	err := ValidateRuntimeEnvironment()
	if err == nil || !strings.Contains(err.Error(), "SLAN_TRUSTED_PROXY_CIDRS") {
		t.Fatalf("invalid trusted proxy CIDR error = %v", err)
	}
}

func TestValidateRuntimeEnvironmentRejectsInvalidTokenTTL(t *testing.T) {
	setValidProductionEnvironment(t)
	t.Setenv("SLAN_DEVICE_ACCESS_TOKEN_TTL", "1m")
	err := ValidateRuntimeEnvironment()
	if err == nil || !strings.Contains(err.Error(), "SLAN_DEVICE_ACCESS_TOKEN_TTL") {
		t.Fatalf("invalid token TTL error = %v", err)
	}
}

func TestValidateProductionSecretDoesNotExposeValue(t *testing.T) {
	const secret = "change-me-secret-value-that-must-not-leak"
	err := validateProductionSecret("SECRET_NAME", secret)
	if err == nil {
		t.Fatal("expected forbidden secret to fail validation")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("validation error exposed secret: %v", err)
	}
}

func setValidProductionEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("SLAN_ENV", "production")
	t.Setenv("SLAN_DEVICE_CREDENTIAL_PEPPER", "production-device-pepper-0123456789abcdef")
	t.Setenv("SLAN_DEVICE_CREDENTIAL_PREVIOUS_PEPPERS", "")
	t.Setenv("SLAN_MQTT_PASSWORD_SECRET", "production-mqtt-secret-0123456789abcdef")
	t.Setenv("SLAN_INTERNAL_WIRE_TOKEN", "production-wire-token-0123456789abcdef")
	t.Setenv("SLAN_MQTT_WEBHOOK_TOKEN", "production-mqtt-webhook-0123456789abcdef")
	t.Setenv("SLAN_SERVICE_BIZ_DSN", "")
	t.Setenv("SLAN_SERVICE_BIZ_DB_PASSWORD", "production-db-password-0123456789")
	t.Setenv("SLAN_MQTT_PUBLIC_BROKER_URL", "mqtts://mqtt.example.com:8883")
	t.Setenv("SLAN_TRUSTED_PROXY_CIDRS", "")
	for _, key := range []string{
		"SLAN_DEVICE_ACCESS_TOKEN_TTL",
		"SLAN_DEVICE_REFRESH_SHORT_TTL",
		"SLAN_DEVICE_REFRESH_LONG_TTL",
		"SLAN_DEVICE_REFRESH_MANUAL_TTL",
		"SLAN_OPS_SESSION_TTL",
	} {
		t.Setenv(key, "")
	}
}
