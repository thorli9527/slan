package config

import (
	"fmt"
	"os"
	"runtime"
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
	PacketWorkers     int
	ReadBufferBytes   int
	WriteBufferBytes  int
	ForwardAckEnabled bool
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
		PacketWorkers:     envInt("SLAN_WIRE_RELAY_PACKET_WORKERS", defaultPacketWorkers()),
		ReadBufferBytes:   envInt("SLAN_WIRE_RELAY_READ_BUFFER_BYTES", 16*1024*1024),
		WriteBufferBytes:  envInt("SLAN_WIRE_RELAY_WRITE_BUFFER_BYTES", 16*1024*1024),
		ForwardAckEnabled: envBool("SLAN_WIRE_RELAY_FORWARD_ACK_ENABLED", false),
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
		problems = append(problems, "SLAN_WIRE_RELAY_PUBLIC_HOST must not be loopback")
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

func defaultPacketWorkers() int {
	workers := runtime.GOMAXPROCS(0)
	if workers < 2 {
		return 2
	}
	if workers > 8 {
		return 8
	}
	return workers
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
