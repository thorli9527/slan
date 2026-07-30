package mqttkit

import "testing"

func TestAllowTopicAccessAllowsDevicePublishToNetworkBroadcast(t *testing.T) {
	cfg := DefaultConfig()

	allowed := AllowTopicAccess(
		cfg,
		"device",
		"device-1",
		"slan/networks/net-1/broadcast",
		false,
	)

	if !allowed {
		t.Fatalf("expected device publish to network broadcast to be allowed")
	}
}

func TestAllowTopicAccessAllowsDevicePublishToTargetDeviceControlDown(t *testing.T) {
	cfg := DefaultConfig()

	allowed := AllowTopicAccess(
		cfg,
		"device",
		"device-1",
		"slan/devices/device-2/control/down",
		false,
	)

	if !allowed {
		t.Fatalf("expected device publish to target device control/down to be allowed")
	}
}

func TestAllowTopicAccessAllowsServerSubscribeToControlAck(t *testing.T) {
	cfg := DefaultConfig()

	if !AllowTopicAccess(cfg, "server", "", "slan/devices/+/control/ack", true) {
		t.Fatalf("expected server subscribe to control/ack wildcard to be allowed")
	}
	if AllowTopicAccess(cfg, "device", "device-1", "slan/devices/+/control/ack", true) {
		t.Fatalf("expected device subscribe to control/ack wildcard to be denied")
	}
}
