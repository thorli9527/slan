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
