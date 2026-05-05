package config

import "os"

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
