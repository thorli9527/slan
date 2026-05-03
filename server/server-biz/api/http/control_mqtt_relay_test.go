package httpapi

import (
	"encoding/json"
	"testing"

	controlmsg "github.com/slan/server/server-biz/internal/controlmsg"
)

func TestParseRelayHeartbeatTopic(t *testing.T) {
	nodeID, ok := parseRelayHeartbeatTopic("slan/devices", "slan/devices/relays/relay-cn-local-udp/heartbeat")
	if !ok {
		t.Fatal("expected relay heartbeat topic to parse")
	}
	if nodeID != "relay-cn-local-udp" {
		t.Fatalf("unexpected node id %q", nodeID)
	}
}

func TestParseRelayHeartbeatTopicRejectsOtherTopics(t *testing.T) {
	if _, ok := parseRelayHeartbeatTopic("slan/devices", "slan/devices/dev-1/control/up"); ok {
		t.Fatal("expected control topic to be rejected")
	}
}

func TestDecodeRelayHeartbeatUsesTopicNodeID(t *testing.T) {
	payload, err := json.Marshal(controlmsg.RelayNodeHeartbeat{
		NodeID:         "spoofed-node",
		ClusterID:      "cn-local-a",
		Healthy:        true,
		ActiveSessions: 3,
	})
	if err != nil {
		t.Fatal(err)
	}

	heartbeat, ok, err := decodeRelayHeartbeatMQTTMessage(
		"slan/devices",
		"slan/devices/relays/relay-cn-local-udp/heartbeat",
		payload,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected heartbeat to decode")
	}
	if heartbeat.NodeID != "relay-cn-local-udp" {
		t.Fatalf("expected topic node id to win, got %q", heartbeat.NodeID)
	}
	if heartbeat.ClusterID != "cn-local-a" || heartbeat.ActiveSessions != 3 {
		t.Fatalf("unexpected decoded heartbeat: %#v", heartbeat)
	}
}

func TestNormalizeOpsRelayPolicyPathTypeAcceptsCanonicalPaths(t *testing.T) {
	for _, value := range []string{"direct_udp", "relay_udp", "relay_tcp", "relay_http3", "relay_tls"} {
		if got := normalizeOpsRelayPolicyPathType(value); got != value {
			t.Fatalf("expected %q to be accepted, got %q", value, got)
		}
	}
	if got := normalizeOpsRelayPolicyPathType("quic"); got != "" {
		t.Fatalf("expected unsupported path to be rejected, got %q", got)
	}
}
