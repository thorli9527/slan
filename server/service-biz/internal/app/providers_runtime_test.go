package app

import (
	"testing"
	"time"
)

func TestNewMQTTConfigSeparatesInternalAndPublicBrokerURL(t *testing.T) {
	t.Setenv("SLAN_MQTT_ENABLED", "true")
	t.Setenv("SLAN_MQTT_BROKER_URL", "mqtt://bifromq:1883")
	t.Setenv("SLAN_MQTT_PUBLIC_BROKER_URL", "mqtt://47.245.40.231:1883")
	t.Setenv("SLAN_MQTT_PASSWORD_SECRET", "remote-mqtt-secret")
	t.Setenv("SLAN_MQTT_CREDENTIAL_TTL_MILLISECONDS", "15000")

	cfg := newMQTTConfig()

	if !cfg.Enabled {
		t.Fatalf("expected mqtt enabled")
	}
	if cfg.BrokerURL != "mqtt://bifromq:1883" {
		t.Fatalf("expected internal broker url, got %q", cfg.BrokerURL)
	}
	if cfg.PublicBrokerURL != "mqtt://47.245.40.231:1883" {
		t.Fatalf("expected public broker url, got %q", cfg.PublicBrokerURL)
	}
	if cfg.Secret != "remote-mqtt-secret" {
		t.Fatalf("expected secret to come from env, got %q", cfg.Secret)
	}
	if cfg.CredentialTTL != 15*time.Second {
		t.Fatalf("expected credential ttl 15s, got %s", cfg.CredentialTTL)
	}
}

func TestNewMQTTConfigFallsBackPublicBrokerURLToInternal(t *testing.T) {
	t.Setenv("SLAN_MQTT_ENABLED", "true")
	t.Setenv("SLAN_MQTT_BROKER_URL", "mqtt://bifromq:1883")
	t.Setenv("SLAN_MQTT_PUBLIC_BROKER_URL", "")

	cfg := newMQTTConfig()

	if cfg.BrokerURL != "mqtt://bifromq:1883" {
		t.Fatalf("expected internal broker url, got %q", cfg.BrokerURL)
	}
	if cfg.PublicBrokerURL != "mqtt://bifromq:1883" {
		t.Fatalf("expected public broker url to fall back to internal, got %q", cfg.PublicBrokerURL)
	}
}
