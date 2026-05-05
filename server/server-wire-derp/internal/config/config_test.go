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
	cfg := Config{PublicHost: "127.0.0.1"}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production config validation to reject missing and dev fallback secrets")
	}
}

func TestValidateProductionAcceptsExplicitConfig(t *testing.T) {
	t.Setenv("SLAN_ENV", "production")
	t.Setenv("SLAN_WIRE_TICKET_SECRET", "prod-current-secret")
	t.Setenv("SLAN_WIRE_TICKET_SECRETS", "prod-current-secret,prod-previous-secret")
	cfg := Config{
		BizURL:            "http://server-biz:8080",
		InternalWireToken: "prod-wire-token",
		RegionID:          "cn-east",
		NodeID:            "derp-a",
		PublicHost:        "derp.example.com",
		PublicPort:        443,
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
		BizURL:            "http://server-biz:8080",
		InternalWireToken: "prod-wire-token",
		RegionID:          "cn-east",
		NodeID:            "derp-a",
		PublicHost:        "derp.example.com",
		PublicPort:        443,
	}

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected production config validation to reject signing key outside key ring")
	}
}
