package config

import "testing"

func TestValidateAllowsDevelopmentDefaults(t *testing.T) {
	cfg := Load()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateProductionRejectsDevFallbacks(t *testing.T) {
	t.Setenv("SLAN_ENV", "production")
	t.Setenv("SLAN_WIRE_TICKET_SECRET", "dev-wire-ticket-secret")
	t.Setenv("SLAN_WIRE_TICKET_SECRETS", "dev-wire-ticket-secret")
	cfg := Config{}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production config validation to reject missing and dev fallback secrets")
	}
}

func TestValidateProductionAcceptsExplicitConfig(t *testing.T) {
	t.Setenv("SLAN_ENV", "production")
	t.Setenv("SLAN_WIRE_TICKET_SECRET", "prod-current-secret")
	t.Setenv("SLAN_WIRE_TICKET_SECRETS", "prod-current-secret,prod-previous-secret")
	cfg := Config{
		BizInternalURL:   "http://server-biz:8080",
		BizInternalToken: "prod-wire-token",
		PostgresDSN:      "postgres://postgres:prod-password@postgres:5432/slan?sslmode=disable",
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateProductionRejectsSigningKeyOutsideKeyRing(t *testing.T) {
	t.Setenv("SLAN_ENV", "production")
	t.Setenv("SLAN_WIRE_TICKET_SECRET", "prod-current-secret")
	t.Setenv("SLAN_WIRE_TICKET_SECRETS", "other-current-secret,prod-previous-secret")
	cfg := Config{
		BizInternalURL:   "http://server-biz:8080",
		BizInternalToken: "prod-wire-token",
		PostgresDSN:      "postgres://postgres:prod-password@postgres:5432/slan?sslmode=disable",
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production config validation to reject signing key outside key ring")
	}
}
