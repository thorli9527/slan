package serverbiztest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/slan/server/server-biz/internal/infra"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	payload := []byte(`
http:
  address: ":18080"
ws:
  path: "/control/custom-ws"
relay:
  region: "ap-east"
  udp_endpoint: "203.0.113.10:9000"
  tcp_endpoint: "203.0.113.10:9443"
bootstrap:
  stun_servers:
    - "stun:example.org:3478"
`)
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := infra.LoadConfig(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.HTTP.Address != ":18080" {
		t.Fatalf("unexpected http address: %q", cfg.HTTP.Address)
	}
	if cfg.WS.Path != "/control/custom-ws" {
		t.Fatalf("unexpected ws path: %q", cfg.WS.Path)
	}
	if cfg.Relay.Region != "ap-east" || cfg.Relay.UDPEndpoint != "203.0.113.10:9000" || cfg.Relay.TCPEndpoint != "203.0.113.10:9443" {
		t.Fatalf("unexpected relay config: %+v", cfg.Relay)
	}
	if len(cfg.Bootstrap.STUNServers) != 1 || cfg.Bootstrap.STUNServers[0] != "stun:example.org:3478" {
		t.Fatalf("unexpected stun servers: %+v", cfg.Bootstrap.STUNServers)
	}
}
