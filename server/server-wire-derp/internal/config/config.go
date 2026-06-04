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
	AdminListenAddr   string
	BizURL            string
	InternalWireToken string
	RegionID          string
	NodeID            string
	PublicHost        string
	PublicPort        int
	Priority          int
	Enabled           bool
	HeartbeatInterval time.Duration
	ReadBufferBytes   int
	WriteBufferBytes  int
	SendAckEnabled    bool
}

func Load() Config {
	addr := os.Getenv("SLAN_WIRE_DERP_LISTEN_ADDR")
	if addr == "" {
		addr = "127.0.0.1:29120"
	}
	adminAddr := os.Getenv("SLAN_WIRE_DERP_ADMIN_LISTEN_ADDR")
	if adminAddr == "" {
		adminAddr = "127.0.0.1:29121"
	}
	return Config{
		ListenAddr:        addr,
		AdminListenAddr:   adminAddr,
		BizURL:            env("SLAN_BIZ_URL", ""),
		InternalWireToken: env("SLAN_INTERNAL_WIRE_TOKEN", ""),
		RegionID:          env("SLAN_WIRE_DERP_REGION_ID", "local"),
		NodeID:            env("SLAN_WIRE_DERP_NODE_ID", "derp-local"),
		PublicHost:        env("SLAN_WIRE_DERP_PUBLIC_HOST", listenHost(addr)),
		PublicPort:        envInt("SLAN_WIRE_DERP_PUBLIC_PORT", listenPort(addr, 29120)),
		Priority:          envInt("SLAN_WIRE_DERP_PRIORITY", 100),
		Enabled:           envBool("SLAN_WIRE_DERP_ENABLED", true),
		HeartbeatInterval: time.Duration(envInt("SLAN_WIRE_DERP_HEARTBEAT_SECONDS", 30)) * time.Second,
		ReadBufferBytes:   envInt("SLAN_WIRE_DERP_READ_BUFFER_BYTES", 4*1024*1024),
		WriteBufferBytes:  envInt("SLAN_WIRE_DERP_WRITE_BUFFER_BYTES", 4*1024*1024),
		SendAckEnabled:    envBool("SLAN_WIRE_DERP_SEND_ACK_ENABLED", false),
	}
}

func (c Config) Validate() error {
	if !isProductionEnv() {
		return nil
	}
	var problems []string
	if strings.TrimSpace(c.BizURL) == "" {
		problems = append(problems, "SLAN_BIZ_URL is required")
	}
	if weakSecret(c.InternalWireToken) {
		problems = append(problems, "SLAN_INTERNAL_WIRE_TOKEN must be set to a production secret")
	}
	if strings.TrimSpace(c.RegionID) == "" || strings.TrimSpace(c.NodeID) == "" {
		problems = append(problems, "region and node id are required")
	}
	if isLoopbackHost(c.PublicHost) {
		problems = append(problems, "SLAN_WIRE_DERP_PUBLIC_HOST must not be loopback")
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

func isLoopbackHost(value string) bool {
	host := strings.ToLower(strings.TrimSpace(value))
	return host == "" || host == "localhost" || host == "::1" || strings.HasPrefix(host, "127.")
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

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return strings.EqualFold(value, "true") || value == "1" || strings.EqualFold(value, "yes")
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
		if port, err := strconv.Atoi(strings.TrimSpace(addr[idx+1:])); err == nil && port > 0 {
			return port
		}
	}
	return fallback
}
