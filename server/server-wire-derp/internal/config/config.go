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
	PublicPort        int
	Priority          int
	Enabled           bool
	HeartbeatInterval time.Duration
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
		PublicPort:        envInt("SLAN_WIRE_DERP_PUBLIC_PORT", 443),
		Priority:          envInt("SLAN_WIRE_DERP_PRIORITY", 100),
		Enabled:           envBool("SLAN_WIRE_DERP_ENABLED", true),
		HeartbeatInterval: time.Duration(envInt("SLAN_WIRE_DERP_HEARTBEAT_SECONDS", 30)) * time.Second,
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
