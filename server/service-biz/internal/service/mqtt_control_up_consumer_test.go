package service

import (
	"context"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
)

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

func TestReportEndpointAppliesDeviceEndpointsToEveryActiveNetwork(t *testing.T) {
	networks := &networkRuntimeTestNetworks{
		networks: map[string]model.Network{
			"net-a": {NetworkID: "net-a", Status: "active"},
			"net-b": {NetworkID: "net-b", Status: "active"},
		},
		networkDevices: map[string][]model.NetworkDevice{
			"net-a": {{NetworkID: "net-a", DeviceID: "device-a", Enabled: true, MemberStatus: model.NetworkMemberStatusActive}},
			"net-b": {{NetworkID: "net-b", DeviceID: "device-a", Enabled: true, MemberStatus: model.NetworkMemberStatusActive}},
		},
	}
	service := MQTTWebhookService{
		Networks: networks,
		Now:      func() time.Time { return time.Unix(1700003000, 0) },
	}

	changed, err := service.ReportEndpoint(context.Background(), MQTTEndpointReportInput{
		NetworkID: "net-a",
		DeviceID:  "device-a",
		NodeID:    "node-device-a",
		NATType:   "easy",
		Endpoints: []DeviceEndpointView{{Type: "direct_udp", Address: "1.2.3.4:5678"}},
	})
	if err != nil {
		t.Fatalf("ReportEndpoint returned error: %v", err)
	}
	if !changed {
		t.Fatal("expected endpoint change")
	}
	if got := len(networks.savedNetworkDevices); got != 2 {
		t.Fatalf("expected endpoints saved to 2 networks, got %d", got)
	}
}

func TestDeviceEndpointsChangedIgnoresTimestampAndOrder(t *testing.T) {
	previous := []model.DeviceEndpoint{
		{Type: "direct_udp", Address: "1.2.3.4:5678", UpdatedAt: 100},
		{Type: "lan_udp", Address: "192.168.1.2:5678", UpdatedAt: 100},
	}
	next := []model.DeviceEndpoint{
		{Type: "lan_udp", Address: "192.168.1.2:5678", UpdatedAt: 200},
		{Type: "direct_udp", Address: "1.2.3.4:5678", UpdatedAt: 200},
	}

	if deviceEndpointsChanged(previous, next) {
		t.Fatal("timestamp and ordering changes must not trigger network reconfiguration")
	}
	next[1].Address = "1.2.3.4:9876"
	if !deviceEndpointsChanged(previous, next) {
		t.Fatal("endpoint address changes must trigger network reconfiguration")
	}
}
