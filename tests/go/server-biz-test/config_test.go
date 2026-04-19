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
  default_cluster_id: "cn-bj-a"
  countries:
    - country_code: "CN"
      country_name: "China"
      cities:
        - city_code: "bj"
          city_name: "Beijing"
          clusters:
            - cluster_id: "cn-bj-a"
              cluster_name: "CN Beijing A"
              nodes:
                - node_id: "relay-cn-bj-udp"
                  transport: "udp"
                  address: "203.0.113.10:9000"
                  priority: 10
                - node_id: "relay-cn-bj-tcp"
                  transport: "tcp"
                  address: "203.0.113.10:9443"
                  priority: 20
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
	if cfg.Relay.DefaultClusterID != "cn-bj-a" {
		t.Fatalf("unexpected default relay cluster: %+v", cfg.Relay)
	}
	if len(cfg.Relay.Countries) != 1 || cfg.Relay.Countries[0].CountryCode != "CN" {
		t.Fatalf("unexpected relay countries: %+v", cfg.Relay.Countries)
	}
	nodes := cfg.Relay.Countries[0].Cities[0].Clusters[0].Nodes
	if len(nodes) != 2 || nodes[0].Address != "203.0.113.10:9000" || nodes[1].Address != "203.0.113.10:9443" {
		t.Fatalf("unexpected relay config: %+v", cfg.Relay)
	}
	if len(cfg.Bootstrap.STUNServers) != 1 || cfg.Bootstrap.STUNServers[0] != "stun:example.org:3478" {
		t.Fatalf("unexpected stun servers: %+v", cfg.Bootstrap.STUNServers)
	}
}
