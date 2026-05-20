package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ListenAddr        string
	HTTPListenAddr    string
	PublicHost        string
	PublicUDPPort     int
	InternalWireToken string
	EndpointTTL       time.Duration
	SessionTTL        time.Duration
}

func Load() Config {
	udpAddr := env("SLAN_WIRE_PUNCH_LISTEN_ADDR", ":29130")
	httpAddr := env("SLAN_WIRE_PUNCH_HTTP_LISTEN_ADDR", ":29131")
	return Config{
		ListenAddr:        udpAddr,
		HTTPListenAddr:    httpAddr,
		PublicHost:        env("SLAN_WIRE_PUNCH_PUBLIC_HOST", listenHost(udpAddr)),
		PublicUDPPort:     envInt("SLAN_WIRE_PUNCH_PUBLIC_UDP_PORT", listenPort(udpAddr, 29130)),
		InternalWireToken: env("SLAN_INTERNAL_WIRE_TOKEN", ""),
		EndpointTTL:       time.Duration(envInt("SLAN_WIRE_PUNCH_ENDPOINT_TTL_SECONDS", 120)) * time.Second,
		SessionTTL:        time.Duration(envInt("SLAN_WIRE_PUNCH_SESSION_TTL_SECONDS", 60)) * time.Second,
	}
}

func (c Config) Validate() error {
	var problems []string
	if strings.TrimSpace(c.ListenAddr) == "" {
		problems = append(problems, "SLAN_WIRE_PUNCH_LISTEN_ADDR is required")
	}
	if strings.TrimSpace(c.HTTPListenAddr) == "" {
		problems = append(problems, "SLAN_WIRE_PUNCH_HTTP_LISTEN_ADDR is required")
	}
	if c.EndpointTTL <= 0 {
		problems = append(problems, "endpoint ttl must be positive")
	}
	if c.SessionTTL <= 0 {
		problems = append(problems, "session ttl must be positive")
	}
	if isProductionEnv() {
		if strings.TrimSpace(c.PublicHost) == "" || isLoopbackHost(c.PublicHost) {
			problems = append(problems, "SLAN_WIRE_PUNCH_PUBLIC_HOST must be non-loopback in production")
		}
		if weakSecret(c.InternalWireToken) {
			problems = append(problems, "SLAN_INTERNAL_WIRE_TOKEN must be set to a production secret")
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("invalid punch config: %s", strings.Join(problems, "; "))
	}
	return nil
}

func env(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key))); err == nil && value > 0 {
		return value
	}
	return fallback
}

func listenHost(addr string) string {
	host := addr
	if idx := strings.LastIndex(host, ":"); idx >= 0 {
		host = host[:idx]
	}
	host = strings.Trim(host, "[]")
	if host == "" || host == "0.0.0.0" || host == "::" {
		return "127.0.0.1"
	}
	return host
}

func listenPort(addr string, fallback int) int {
	if idx := strings.LastIndex(addr, ":"); idx >= 0 {
		if port, err := strconv.Atoi(addr[idx+1:]); err == nil && port > 0 {
			return port
		}
	}
	return fallback
}

func isProductionEnv() bool {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("SLAN_ENV")))
	return env == "prod" || env == "production"
}

func weakSecret(value string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	return trimmed == "" || strings.Contains(trimmed, "change-me")
}

func isLoopbackHost(value string) bool {
	host := strings.ToLower(strings.TrimSpace(value))
	return host == "" || host == "localhost" || host == "::1" || strings.HasPrefix(host, "127.")
}
