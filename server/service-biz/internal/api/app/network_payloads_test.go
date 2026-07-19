package app

import (
	"testing"

	servicepkg "github.com/slan/service-biz/internal/service"
)

func TestNetworkResolvedConfigPayloadUsesStoredConfigVersion(t *testing.T) {
	payload := networkResolvedConfigPayload(servicepkg.NetworkResolvedConfigView{
		Config: servicepkg.NetworkConfigView{
			Network: servicepkg.NetworkView{
				NetworkID: "net-1",
				Name:      "Default",
				UpdatedAt: 123,
			},
			ConfigVersion: 456,
			DeviceID:      "dev-1",
			NodeID:        "node-dev-1",
			GlobalIP:      "10.0.0.2",
			PrefixLen:     24,
		},
	})

	got, _ := payload["configVersion"].(int64)
	if got != 456 {
		t.Fatalf("expected configVersion=456, got %v", payload["configVersion"])
	}
}

func TestRuntimeNodeConfigsUseCanonicalPathPriority(t *testing.T) {
	nodes := runtimeNodeConfigs(
		[]servicepkg.PunchNodeView{{
			NodeID:   "punch-1",
			Address:  "47.245.40.231:29130",
			Priority: 5,
		}},
		[]map[string]any{
			{
				"networkId": "net-1",
				"relayCandidates": []map[string]any{
					{"endpointId": "tcp-1", "transport": "derp_tcp_tls_443", "address": "47.245.40.231:29120"},
					{"endpointId": "udp-1", "transport": "udp", "address": "47.245.40.231:29112"},
				},
			},
			{
				"networkId": "net-2",
				"relayCandidates": []map[string]any{
					{"endpointId": "udp-1", "transport": "udp", "address": "47.245.40.231:29112"},
				},
			},
		},
	)

	if len(nodes) != 3 {
		t.Fatalf("expected 3 unique nodes, got %d", len(nodes))
	}
	for index, pathKind := range []string{"direct_udp", "relay_udp", "relay_tcp"} {
		if nodes[index]["pathKind"] != pathKind {
			t.Fatalf("node %d pathKind=%v, want %s", index, nodes[index]["pathKind"], pathKind)
		}
	}
	networkIDs, _ := nodes[1]["networkIds"].([]string)
	if len(networkIDs) != 2 || networkIDs[0] != "net-1" || networkIDs[1] != "net-2" {
		t.Fatalf("unexpected merged networkIds: %#v", networkIDs)
	}
}
