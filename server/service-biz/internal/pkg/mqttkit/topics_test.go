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
