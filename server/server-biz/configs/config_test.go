package configs

import (
	"strings"
	"testing"
)

func TestLoadConfig_AllowsDevelopmentDefaults(t *testing.T) {
	t.Setenv("SLAN_ENV", "")

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("load default config: %v", err)
	}
	if cfg.Ops.AccessToken != DefaultConfig().Ops.AccessToken {
		t.Fatalf("expected development defaults, got %q", cfg.Ops.AccessToken)
	}
	if cfg.Redis.DialTimeoutSeconds != 5 ||
		cfg.Redis.ReadTimeoutSeconds != 3 ||
		cfg.Redis.WriteTimeoutSeconds != 3 {
		t.Fatalf("expected redis timeout defaults, got %+v", cfg.Redis)
	}
	if cfg.Auth.AccessTokenTTLSeconds != 3600 || cfg.Auth.RefreshTokenTTLSeconds != 86400 {
		t.Fatalf("expected auth token ttl defaults, got %+v", cfg.Auth)
	}
}

func TestLoadConfig_AppliesAuthTTLOverrides(t *testing.T) {
	t.Setenv("SLAN_ENV", "")
	t.Setenv("SLAN_ACCESS_TOKEN_TTL_SECONDS", "120")
	t.Setenv("SLAN_REFRESH_TOKEN_TTL_SECONDS", "240")

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("load default config with auth ttl overrides: %v", err)
	}
	if cfg.Auth.AccessTokenTTLSeconds != 120 || cfg.Auth.RefreshTokenTTLSeconds != 240 {
		t.Fatalf("expected auth token ttl overrides, got %+v", cfg.Auth)
	}
}

func TestLoadConfig_RejectsProductionDefaults(t *testing.T) {
	t.Setenv("SLAN_ENV", "production")

	_, err := LoadConfig("")
	if err == nil {
		t.Fatal("expected production config validation error")
	}
	message := err.Error()
	for _, want := range []string{
		"http.public_scheme must be https",
		"http.public_host must not be a loopback host",
		"relay.ticket_signing_secret must be replaced",
		"ops.access_token must be replaced",
		"ops.default_admin.password must be replaced or default admin disabled",
		"postgres.password must be replaced",
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("expected error to contain %q, got %q", want, message)
		}
	}
}

func TestLoadConfig_AcceptsProductionEnvOverrides(t *testing.T) {
	t.Setenv("SLAN_ENV", "production")
	t.Setenv("SLAN_HTTP_PUBLIC_HOST", "slan.example.com")
	t.Setenv("SLAN_HTTP_PUBLIC_SCHEME", "https")
	t.Setenv("SLAN_RELAY_TICKET_SIGNING_SECRET", "relay-secret-with-enough-entropy")
	t.Setenv("SLAN_OPS_ACCESS_TOKEN", "ops-token-with-enough-entropy")
	t.Setenv("SLAN_OPS_DEFAULT_ADMIN_PASSWORD", "Admin-Password-2026")
	t.Setenv("SLAN_POSTGRES_PASSWORD", "postgres-password-with-enough-entropy")

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("load production config with overrides: %v", err)
	}
	if cfg.HTTP.PublicHost != "slan.example.com" || cfg.HTTP.PublicScheme != "https" {
		t.Fatalf("expected production public endpoint overrides, got %+v", cfg.HTTP)
	}
}
