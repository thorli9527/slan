package config

import (
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
	PublicUDPPort     int
	PublicAdminPort   int
	Priority          int
	Enabled           bool
	HeartbeatInterval time.Duration
}

func Load() Config {
	addr := os.Getenv("SLAN_WIRE_RELAY_LISTEN_ADDR")
	if addr == "" {
		addr = "127.0.0.1:29110"
	}
	adminAddr := os.Getenv("SLAN_WIRE_RELAY_ADMIN_LISTEN_ADDR")
	if adminAddr == "" {
		adminAddr = "127.0.0.1:29111"
	}
	return Config{
		ListenAddr:        addr,
		AdminListenAddr:   adminAddr,
		BizURL:            env("SLAN_BIZ_URL", ""),
		InternalWireToken: env("SLAN_INTERNAL_WIRE_TOKEN", ""),
		RegionID:          env("SLAN_WIRE_RELAY_REGION_ID", "local"),
		NodeID:            env("SLAN_WIRE_RELAY_NODE_ID", "relay-local"),
		PublicHost:        env("SLAN_WIRE_RELAY_PUBLIC_HOST", listenHost(addr)),
		PublicUDPPort:     envInt("SLAN_WIRE_RELAY_PUBLIC_UDP_PORT", listenPort(addr, 29110)),
		PublicAdminPort:   envInt("SLAN_WIRE_RELAY_PUBLIC_ADMIN_PORT", listenPort(adminAddr, 29111)),
		Priority:          envInt("SLAN_WIRE_RELAY_PRIORITY", 100),
		Enabled:           envBool("SLAN_WIRE_RELAY_ENABLED", true),
		HeartbeatInterval: time.Duration(envInt("SLAN_WIRE_RELAY_HEARTBEAT_SECONDS", 30)) * time.Second,
	}
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
		if port, err := strconv.Atoi(addr[idx+1:]); err == nil && port > 0 {
			return port
		}
	}
	return fallback
}
