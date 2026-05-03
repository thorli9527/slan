package configs

import "testing"

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
