package configs

import (
	"os"
	"path/filepath"
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

func TestLoadConfig_BackfillsClientFacingDefaultsForLegacyConfig(t *testing.T) {
	t.Setenv("SLAN_ENV", "")

	path := filepath.Join(t.TempDir(), "legacy.yaml")
	if err := os.WriteFile(path, []byte(`
http:
  address: ":18080"
postgres:
  host: "postgres"
  password: "legacy-postgres-password"
redis:
  host: "redis"
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load legacy config: %v", err)
	}
	defaults := DefaultConfig()
	if cfg.HTTP.Address != ":18080" {
		t.Fatalf("expected explicit http address to survive, got %+v", cfg.HTTP)
	}
	if cfg.HTTP.OpsAddress != defaults.HTTP.OpsAddress ||
		cfg.HTTP.PublicHost != defaults.HTTP.PublicHost ||
		cfg.HTTP.PublicScheme != defaults.HTTP.PublicScheme {
		t.Fatalf("expected missing public http fields to use defaults, got %+v", cfg.HTTP)
	}
	if cfg.Relay.DefaultClusterID != defaults.Relay.DefaultClusterID ||
		cfg.Relay.TicketSigningSecret != defaults.Relay.TicketSigningSecret ||
		len(cfg.Relay.Countries) == 0 {
		t.Fatalf("expected relay defaults for legacy config, got %+v", cfg.Relay)
	}
	if len(cfg.Bootstrap.STUNServers) == 0 ||
		cfg.Bootstrap.STUNServers[0] != defaults.Bootstrap.STUNServers[0] {
		t.Fatalf("expected bootstrap stun defaults, got %+v", cfg.Bootstrap)
	}
	if cfg.Auth.AccessTokenTTLSeconds != defaults.Auth.AccessTokenTTLSeconds ||
		cfg.Auth.RefreshTokenTTLSeconds != defaults.Auth.RefreshTokenTTLSeconds {
		t.Fatalf("expected auth ttl defaults, got %+v", cfg.Auth)
	}
	if cfg.Postgres.Host != "postgres" || cfg.Postgres.Password != "legacy-postgres-password" {
		t.Fatalf("expected explicit postgres fields to survive, got %+v", cfg.Postgres)
	}
	if cfg.Postgres.Port != defaults.Postgres.Port ||
		cfg.Postgres.Database != defaults.Postgres.Database ||
		cfg.Postgres.Username != defaults.Postgres.Username ||
		cfg.Postgres.SSLMode != defaults.Postgres.SSLMode ||
		cfg.Postgres.MaxOpenConns != defaults.Postgres.MaxOpenConns {
		t.Fatalf("expected missing postgres fields to use defaults, got %+v", cfg.Postgres)
	}
	if cfg.Redis.Host != "redis" ||
		cfg.Redis.Port != defaults.Redis.Port ||
		cfg.Redis.PoolSize != defaults.Redis.PoolSize ||
		cfg.Redis.DialTimeoutSeconds != defaults.Redis.DialTimeoutSeconds {
		t.Fatalf("expected redis compatibility defaults, got %+v", cfg.Redis)
	}
}

func TestLoadConfig_PreservesPartialClientFacingConfig(t *testing.T) {
	t.Setenv("SLAN_ENV", "")

	path := filepath.Join(t.TempDir(), "partial.yaml")
	if err := os.WriteFile(path, []byte(`
http:
  public_host: "control.example.test"
  public_scheme: "https"
relay:
  ticket_signing_secret: "custom-ticket-secret"
auth:
  access_token_ttl_seconds: 7200
bootstrap:
  stun_servers:
    - "stun:custom.example.test:3478"
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load partial config: %v", err)
	}
	defaults := DefaultConfig()
	if cfg.HTTP.PublicHost != "control.example.test" ||
		cfg.HTTP.PublicScheme != "https" {
		t.Fatalf("expected explicit public endpoint fields to survive, got http=%+v", cfg.HTTP)
	}
	if cfg.Relay.TicketSigningSecret != "custom-ticket-secret" {
		t.Fatalf("expected custom relay signing secret, got %+v", cfg.Relay)
	}
	if cfg.Relay.DefaultClusterID != defaults.Relay.DefaultClusterID ||
		len(cfg.Relay.Countries) == 0 {
		t.Fatalf("expected missing relay topology fields to use defaults, got %+v", cfg.Relay)
	}
	if len(cfg.Bootstrap.STUNServers) != 1 || cfg.Bootstrap.STUNServers[0] != "stun:custom.example.test:3478" {
		t.Fatalf("expected custom stun servers, got %+v", cfg.Bootstrap)
	}
	if cfg.Auth.AccessTokenTTLSeconds != 7200 ||
		cfg.Auth.RefreshTokenTTLSeconds != defaults.Auth.RefreshTokenTTLSeconds {
		t.Fatalf("expected partial auth ttl defaults, got %+v", cfg.Auth)
	}
}

func TestLoadConfig_EnvironmentOverridesBackfilledPublicEndpoint(t *testing.T) {
	t.Setenv("SLAN_ENV", "")
	t.Setenv("SLAN_HTTP_PUBLIC_HOST", "env-control.example.test")
	t.Setenv("SLAN_HTTP_PUBLIC_SCHEME", "https")

	path := filepath.Join(t.TempDir(), "legacy.yaml")
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.HTTP.PublicHost != "env-control.example.test" || cfg.HTTP.PublicScheme != "https" {
		t.Fatalf("expected env public endpoint override, got %+v", cfg.HTTP)
	}
}

func TestLoadConfig_AppliesOpsLoginRateLimitConfig(t *testing.T) {
	t.Setenv("SLAN_ENV", "")

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(`
ops:
  access_token: custom-ops-token
  login_rate_limit:
    disabled: true
    window_seconds: 30
    ip_limit: 2
    ip_login_name_limit: 1
    login_name_limit: 3
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Ops.AccessToken != "custom-ops-token" {
		t.Fatalf("expected ops access token override, got %q", cfg.Ops.AccessToken)
	}
	rateLimit := cfg.Ops.LoginRateLimit
	if !rateLimit.Disabled ||
		rateLimit.WindowSeconds != 30 ||
		rateLimit.IPLimit != 2 ||
		rateLimit.IPLoginNameLimit != 1 ||
		rateLimit.LoginNameLimit != 3 {
		t.Fatalf("expected ops login rate limit override, got %+v", rateLimit)
	}
}

func TestLoadConfig_PartialOpsLoginRateLimitKeepsEnabledDefault(t *testing.T) {
	t.Setenv("SLAN_ENV", "")

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(`
ops:
  login_rate_limit:
    window_seconds: 45
`), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	rateLimit := cfg.Ops.LoginRateLimit
	if rateLimit.Disabled {
		t.Fatalf("expected partial ops login rate limit config to keep rate limiting enabled, got %+v", rateLimit)
	}
	if rateLimit.WindowSeconds != 45 {
		t.Fatalf("expected custom window, got %+v", rateLimit)
	}
	if rateLimit.IPLimit != DefaultConfig().Ops.LoginRateLimit.IPLimit ||
		rateLimit.IPLoginNameLimit != DefaultConfig().Ops.LoginRateLimit.IPLoginNameLimit ||
		rateLimit.LoginNameLimit != DefaultConfig().Ops.LoginRateLimit.LoginNameLimit {
		t.Fatalf("expected missing rate limit values to use defaults, got %+v", rateLimit)
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
