package app

import (
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
	servicepkg "github.com/slan/service-biz/internal/service"
)

const minimumProductionSecretLength = 32

func ValidateRuntimeEnvironment() error {
	environment := strings.ToLower(envString("SLAN_ENV"))
	if environment != "prod" && environment != "production" {
		return nil
	}
	checks := []struct {
		name      string
		value     string
		forbidden []string
	}{
		{
			name:      "SLAN_DEVICE_CREDENTIAL_PEPPER",
			value:     envString("SLAN_DEVICE_CREDENTIAL_PEPPER"),
			forbidden: []string{"slan-development-device-credential-pepper"},
		},
		{
			name:      "SLAN_MQTT_PASSWORD_SECRET",
			value:     envString("SLAN_MQTT_PASSWORD_SECRET"),
			forbidden: []string{"slan-dev-secret"},
		},
		{
			name:      "SLAN_INTERNAL_WIRE_TOKEN",
			value:     envString("SLAN_INTERNAL_WIRE_TOKEN"),
			forbidden: []string{"dev-internal-wire-token"},
		},
		{
			name:      "SLAN_MQTT_WEBHOOK_TOKEN",
			value:     envString("SLAN_MQTT_WEBHOOK_TOKEN"),
			forbidden: []string{"dev-mqtt-webhook-token"},
		},
	}
	for _, check := range checks {
		if err := validateProductionSecret(check.name, check.value, check.forbidden...); err != nil {
			return err
		}
	}
	previousPeppers := deviceCredentialPreviousPeppers()
	if len(previousPeppers) > 3 {
		return fmt.Errorf("SLAN_DEVICE_CREDENTIAL_PREVIOUS_PEPPERS must contain at most 3 values in production")
	}
	for _, pepper := range previousPeppers {
		if err := validateProductionSecret("SLAN_DEVICE_CREDENTIAL_PREVIOUS_PEPPERS", pepper, "slan-development-device-credential-pepper"); err != nil {
			return err
		}
	}
	if err := validateProductionDatabase(); err != nil {
		return err
	}
	if err := validateProductionMQTTEndpoint(envString("SLAN_MQTT_PUBLIC_BROKER_URL")); err != nil {
		return err
	}
	if err := validateTrustedProxyCIDRs(envString("SLAN_TRUSTED_PROXY_CIDRS")); err != nil {
		return err
	}
	if err := servicepkg.ValidateTokenTTLConfiguration(); err != nil {
		return err
	}
	return nil
}

func validateTrustedProxyCIDRs(value string) error {
	for _, raw := range strings.Split(value, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if _, _, err := net.ParseCIDR(raw); err != nil {
			return fmt.Errorf("SLAN_TRUSTED_PROXY_CIDRS must contain valid CIDR values")
		}
	}
	return nil
}

func validateProductionSecret(name, value string, forbidden ...string) error {
	value = strings.TrimSpace(value)
	if len(value) < minimumProductionSecretLength {
		return fmt.Errorf("%s must be explicitly configured with at least %d characters in production", name, minimumProductionSecretLength)
	}
	normalized := strings.ToLower(value)
	if strings.Contains(normalized, "change-me") {
		return fmt.Errorf("%s uses a forbidden production value", name)
	}
	for _, candidate := range forbidden {
		if normalized == strings.ToLower(strings.TrimSpace(candidate)) {
			return fmt.Errorf("%s uses a development default in production", name)
		}
	}
	return nil
}

func validateProductionDatabase() error {
	dsn := envString("SLAN_SERVICE_BIZ_DSN")
	if dsn == "" {
		return validateProductionDatabasePassword(envString("SLAN_SERVICE_BIZ_DB_PASSWORD"))
	}
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("SLAN_SERVICE_BIZ_DSN is invalid")
	}
	return validateProductionDatabasePassword(config.Password)
}

func validateProductionDatabasePassword(password string) error {
	password = strings.TrimSpace(password)
	if len(password) < 16 {
		return fmt.Errorf("PostgreSQL password must be explicitly configured with at least 16 characters in production")
	}
	normalized := strings.ToLower(password)
	for _, forbidden := range []string{"slan", "postgres", "password"} {
		if normalized == forbidden || strings.Contains(normalized, "change-me") {
			return fmt.Errorf("PostgreSQL password uses a forbidden production value")
		}
	}
	return nil
}

func validateProductionMQTTEndpoint(rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Hostname() == "" {
		return fmt.Errorf("SLAN_MQTT_PUBLIC_BROKER_URL must be explicitly configured in production")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "mqtts" && scheme != "wss" {
		return fmt.Errorf("SLAN_MQTT_PUBLIC_BROKER_URL must use mqtts or wss in production")
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "localhost" {
		return fmt.Errorf("SLAN_MQTT_PUBLIC_BROKER_URL must not use a loopback host in production")
	}
	if address := net.ParseIP(host); address != nil && address.IsLoopback() {
		return fmt.Errorf("SLAN_MQTT_PUBLIC_BROKER_URL must not use a loopback host in production")
	}
	return nil
}
