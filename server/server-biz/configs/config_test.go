package configs

import (
	"os"
	"testing"
)

func TestValidateRelayTopologyAcceptsDefaultConfig(t *testing.T) {
	relay := DefaultConfig().Relay
	if err := validateRelayTopology(&relay); err != nil {
		t.Fatalf("expected default relay topology to be valid: %v", err)
	}
}

func TestValidateRelayTopologyRejectsDuplicateRelayNodeID(t *testing.T) {
	relay := DefaultConfig().Relay
	relay.Countries[0].Cities[0].Clusters[0].Nodes = append(
		relay.Countries[0].Cities[0].Clusters[0].Nodes,
		relay.Countries[0].Cities[0].Clusters[0].Nodes[0],
	)

	if err := validateRelayTopology(&relay); err == nil {
		t.Fatal("expected duplicate relay node id to be rejected")
	}
}

func TestValidateRelayTopologyRejectsMissingDefaultCluster(t *testing.T) {
	relay := DefaultConfig().Relay
	relay.DefaultClusterID = "missing-cluster"

	if err := validateRelayTopology(&relay); err == nil {
		t.Fatal("expected missing default relay cluster to be rejected")
	}
}

func TestValidateRelayTopologyRejectsUnsupportedTransport(t *testing.T) {
	relay := DefaultConfig().Relay
	relay.Countries[0].Cities[0].Clusters[0].Nodes[0].Transport = "smtp"

	if err := validateRelayTopology(&relay); err == nil {
		t.Fatal("expected unsupported relay transport to be rejected")
	}
}

func TestValidateRelayTopologyAcceptsHttp3Transport(t *testing.T) {
	relay := DefaultConfig().Relay
	relay.Countries[0].Cities[0].Clusters[0].Nodes[0].Transport = "http3"

	if err := validateRelayTopology(&relay); err != nil {
		t.Fatalf("expected http3 to be accepted: %v", err)
	}
	if got := relay.Countries[0].Cities[0].Clusters[0].Nodes[0].Transport; got != "http3" {
		t.Fatalf("expected http3 to stay http3, got %s", got)
	}
}

func TestValidateRelayTopologyRejectsHttp3Aliases(t *testing.T) {
	for _, value := range []string{"h3", "quic"} {
		relay := DefaultConfig().Relay
		relay.Countries[0].Cities[0].Clusters[0].Nodes[0].Transport = value

		if err := validateRelayTopology(&relay); err == nil {
			t.Fatalf("expected %s to be rejected", value)
		}
	}
}

func TestWireConfigDefaultsAndEnvOverrides(t *testing.T) {
	t.Setenv("SLAN_WIRE_CONTROL_PLANE_URLS", " http://wire-a:29100, http://wire-b:29100 ")
	t.Setenv("SLAN_WIRE_NODE_HEARTBEAT_FRESHNESS_SECONDS", "45")
	t.Setenv("SLAN_WIRE_NODE_CLEANUP_INTERVAL_SECONDS", "15")
	t.Setenv("SLAN_WIRE_NODE_EVENT_RETENTION_SECONDS", "3600")

	cfg, err := LoadConfig("")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Wire.NodeHeartbeatFreshnessSeconds != 45 {
		t.Fatalf("NodeHeartbeatFreshnessSeconds=%d want 45", cfg.Wire.NodeHeartbeatFreshnessSeconds)
	}
	if len(cfg.Wire.ControlPlaneURLs) != 2 || cfg.Wire.ControlPlaneURLs[0] != "http://wire-a:29100" || cfg.Wire.ControlPlaneURLs[1] != "http://wire-b:29100" {
		t.Fatalf("ControlPlaneURLs=%#v", cfg.Wire.ControlPlaneURLs)
	}
	if cfg.Wire.NodeCleanupIntervalSeconds != 15 {
		t.Fatalf("NodeCleanupIntervalSeconds=%d want 15", cfg.Wire.NodeCleanupIntervalSeconds)
	}
	if cfg.Wire.NodeEventRetentionSeconds != 3600 {
		t.Fatalf("NodeEventRetentionSeconds=%d want 3600", cfg.Wire.NodeEventRetentionSeconds)
	}
}

func TestWireConfigYAMLDefaults(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "config-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	_, err = file.WriteString(`relay:
  default_cluster_id: cn-local-a
  ticket_signing_secret: local-secret
  countries:
    - country_code: CN
      country_name: China
      cities:
        - city_code: local
          city_name: Local
          clusters:
            - cluster_id: cn-local-a
              cluster_name: Local
              nodes:
                - node_id: relay-a
                  transport: udp
                  address: 127.0.0.1:9000
wire:
  control_plane_urls:
    - http://wire-yaml:29100
  node_heartbeat_freshness_seconds: 90
  node_event_retention_seconds: 1800
`)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(file.Name())
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.Wire.NodeHeartbeatFreshnessSeconds != 90 {
		t.Fatalf("NodeHeartbeatFreshnessSeconds=%d want 90", cfg.Wire.NodeHeartbeatFreshnessSeconds)
	}
	if len(cfg.Wire.ControlPlaneURLs) != 1 || cfg.Wire.ControlPlaneURLs[0] != "http://wire-yaml:29100" {
		t.Fatalf("ControlPlaneURLs=%#v", cfg.Wire.ControlPlaneURLs)
	}
	if cfg.Wire.NodeCleanupIntervalSeconds != DefaultConfig().Wire.NodeCleanupIntervalSeconds {
		t.Fatalf("NodeCleanupIntervalSeconds=%d want default", cfg.Wire.NodeCleanupIntervalSeconds)
	}
	if cfg.Wire.NodeEventRetentionSeconds != 1800 {
		t.Fatalf("NodeEventRetentionSeconds=%d want 1800", cfg.Wire.NodeEventRetentionSeconds)
	}
}
