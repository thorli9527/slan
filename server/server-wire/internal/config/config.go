package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	ListenAddr       string
	BizInternalURL   string
	BizInternalToken string
	PostgresDSN      string
}

func Load() Config {
	addr := os.Getenv("SLAN_WIRE_LISTEN_ADDR")
	if addr == "" {
		addr = "127.0.0.1:29100"
	}
	return Config{
		ListenAddr:       addr,
		BizInternalURL:   os.Getenv("SLAN_WIRE_BIZ_INTERNAL_URL"),
		BizInternalToken: os.Getenv("SLAN_INTERNAL_WIRE_TOKEN"),
		PostgresDSN:      os.Getenv("SLAN_WIRE_POSTGRES_DSN"),
	}
}

func (c Config) Validate() error {
	if !isProductionEnv() {
		return nil
	}
	var problems []string
	if strings.TrimSpace(c.BizInternalURL) == "" {
		problems = append(problems, "SLAN_WIRE_BIZ_INTERNAL_URL is required")
	}
	if weakSecret(c.BizInternalToken) {
		problems = append(problems, "SLAN_INTERNAL_WIRE_TOKEN must be set to a production secret")
	}
	if strings.TrimSpace(c.PostgresDSN) == "" {
		problems = append(problems, "SLAN_WIRE_POSTGRES_DSN is required")
	}
	if weakSecret(os.Getenv("SLAN_WIRE_TICKET_SECRET")) {
		problems = append(problems, "SLAN_WIRE_TICKET_SECRET must be set to a production secret")
	}
	if weakSecret(os.Getenv("SLAN_WIRE_TICKET_SECRETS")) {
		problems = append(problems, "SLAN_WIRE_TICKET_SECRETS must be set to a production key ring")
	}
	if !signingKeyMatchesKeyRing(os.Getenv("SLAN_WIRE_TICKET_SECRET"), os.Getenv("SLAN_WIRE_TICKET_SECRETS")) {
		problems = append(problems, "SLAN_WIRE_TICKET_SECRET must match the first key in SLAN_WIRE_TICKET_SECRETS")
	}
	if len(problems) > 0 {
		return fmt.Errorf("invalid production config: %s", strings.Join(problems, "; "))
	}
	return nil
}

func isProductionEnv() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("SLAN_ENV")))
	return env == "prod" || env == "production"
}

func weakSecret(value string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	return trimmed == "" ||
		strings.Contains(trimmed, "change-me") ||
		strings.Contains(trimmed, "dev-wire-ticket-secret")
}

func signingKeyMatchesKeyRing(signing string, ring string) bool {
	signing = strings.TrimSpace(signing)
	for _, item := range strings.Split(ring, ",") {
		first := strings.TrimSpace(item)
		if first == "" {
			continue
		}
		return signing != "" && signing == first
	}
	return false
}
