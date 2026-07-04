package service

import "testing"

func TestDecodeControlUpEndpointReport(t *testing.T) {
	t.Parallel()

	input, err := decodeControlUpEndpointReport(
		"slan/devices/dev123/control/up",
		MQTTControlUpEnvelope{
			Type:      "endpoint_report",
			NetworkID: "net123",
			Payload: []byte(`{
				"networkId":"net123",
				"nodeId":"node-dev123",
				"natType":"easy",
				"endpoints":[{"type":"direct_udp","address":"1.2.3.4:5678","updatedAt":123}]
			}`),
		},
	)
	if err != nil {
		t.Fatalf("decodeControlUpEndpointReport() error = %v", err)
	}
	if input.NetworkID != "net123" || input.DeviceID != "dev123" || input.NodeID != "node-dev123" {
		t.Fatalf("unexpected ids: %#v", input)
	}
	if len(input.Endpoints) != 1 || input.Endpoints[0].Address != "1.2.3.4:5678" {
		t.Fatalf("unexpected endpoints: %#v", input.Endpoints)
	}
}

func TestDecodeControlUpPathHealthReport(t *testing.T) {
	t.Parallel()

	input, err := decodeControlUpPathHealthReport(
		"slan/devices/dev123/control/up",
		MQTTControlUpEnvelope{
			Type:      "path_health_report",
			NetworkID: "net123",
			Payload: []byte(`{
				"networkId":"net123",
				"pathType":"relay_tcp",
				"activePath":"relay_tcp",
				"relayTransport":"tcp",
				"endpoint":"relay.example:443",
				"ticketRenewDue":true,
				"sampledAtMs":456
			}`),
		},
	)
	if err != nil {
		t.Fatalf("decodeControlUpPathHealthReport() error = %v", err)
	}
	if input.NetworkID != "net123" || input.DeviceID != "dev123" {
		t.Fatalf("unexpected ids: %#v", input)
	}
	if input.PathType != "relay_tcp" || input.RelayTransport != "tcp" || !input.TicketRenewDue {
		t.Fatalf("unexpected path payload: %#v", input)
	}
}
