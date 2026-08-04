package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
)

func TestHandleUpstreamMessageRejectsOversizedPayload(t *testing.T) {
	t.Parallel()

	service := MQTTWebhookService{}
	err := service.HandleUpstreamMessage(
		context.Background(),
		"slan/devices/device-a/heartbeat",
		[]byte(strings.Repeat("x", maxMQTTUpstreamPayloadBytes+1)),
	)
	if err == nil || !strings.Contains(err.Error(), "payload exceeds") {
		t.Fatalf("expected payload size error, got %v", err)
	}
}

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

func TestHandleUpstreamPresenceMessagesRefreshMembership(t *testing.T) {
	now := time.Unix(1700003000, 0)
	for _, test := range []struct {
		name               string
		topicSuffix        string
		wantHeartbeatAt    int64
		wantRuntimeStateAt int64
	}{
		{name: "heartbeat", topicSuffix: "heartbeat", wantHeartbeatAt: now.Unix()},
		{name: "runtime state", topicSuffix: "runtime-state", wantRuntimeStateAt: now.Unix()},
	} {
		t.Run(test.name, func(t *testing.T) {
			networks := &networkRuntimeTestNetworks{
				networks: map[string]model.Network{
					"net-a": {NetworkID: "net-a", Status: "active"},
				},
				networkDevices: map[string][]model.NetworkDevice{
					"net-a": {{
						NetworkID:      "net-a",
						DeviceID:       "device-a",
						Enabled:        true,
						MemberStatus:   model.NetworkMemberStatusActive,
						PresenceStatus: model.DevicePresenceStatusOffline,
					}},
				},
			}
			service := MQTTWebhookService{
				Networks: networks,
				Now:      func() time.Time { return now },
			}

			err := service.HandleUpstreamMessage(
				context.Background(),
				"slan/devices/device-a/"+test.topicSuffix,
				[]byte(`{"deviceId":"device-a","activeNetworkId":"net-a","reportedAtMs":1700003000000}`),
			)
			if err != nil {
				t.Fatalf("HandleUpstreamMessage returned error: %v", err)
			}
			if len(networks.savedNetworkDevices) != 1 {
				t.Fatalf("expected one membership update, got %d", len(networks.savedNetworkDevices))
			}
			saved := networks.savedNetworkDevices[0]
			if saved.LastSeenAt != now.Unix() || saved.PresenceStatus != model.DevicePresenceStatusActive {
				t.Fatalf("unexpected presence update: %#v", saved)
			}
			if saved.LastHeartbeatAt != test.wantHeartbeatAt || saved.LastRuntimeStateAt != test.wantRuntimeStateAt {
				t.Fatalf("unexpected message timestamp update: %#v", saved)
			}
		})
	}
}

func TestHandleUpstreamPresenceRejectsDeviceMismatch(t *testing.T) {
	service := MQTTWebhookService{Networks: &networkRuntimeTestNetworks{}}
	err := service.HandleUpstreamMessage(
		context.Background(),
		"slan/devices/device-a/heartbeat",
		[]byte(`{"deviceId":"device-b","activeNetworkId":"net-a"}`),
	)
	if err == nil {
		t.Fatal("expected device mismatch error")
	}
}

func TestReportEndpointUpdatesOnlyReportedNetwork(t *testing.T) {
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
	if got := len(networks.savedNetworkDevices); got != 1 {
		t.Fatalf("expected endpoint saved only to reported network, got %d", got)
	}
	saved := networks.savedNetworkDevices[0]
	if saved.NetworkID != "net-a" {
		t.Fatalf("expected net-a update, got %q", saved.NetworkID)
	}
	if saved.LastEndpointAt != 1700003000 || saved.LastSeenAt != 1700003000 {
		t.Fatalf("expected endpoint presence timestamps, got %#v", saved)
	}
	if len(saved.Endpoints) != 1 || saved.Endpoints[0].UpdatedAt != 1700003000 {
		t.Fatalf("expected endpoint timestamp fallback, got %#v", saved.Endpoints)
	}
}

func TestReportEndpointSkipsUnchangedEndpointWrite(t *testing.T) {
	networks := &networkRuntimeTestNetworks{
		networks: map[string]model.Network{
			"net-a": {NetworkID: "net-a", Status: "active"},
		},
		networkDevices: map[string][]model.NetworkDevice{
			"net-a": {{
				NetworkID:    "net-a",
				DeviceID:     "device-a",
				Enabled:      true,
				MemberStatus: model.NetworkMemberStatusActive,
				NATType:      "easy",
				Endpoints: []model.DeviceEndpoint{{
					Type:      "direct_udp",
					Address:   "1.2.3.4:5678",
					UpdatedAt: 100,
				}},
			}},
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
		Endpoints: []DeviceEndpointView{{
			Type:      "direct_udp",
			Address:   "1.2.3.4:5678",
			UpdatedAt: 1700003000,
		}},
	})
	if err != nil {
		t.Fatalf("ReportEndpoint returned error: %v", err)
	}
	if changed {
		t.Fatal("unchanged endpoint must not report a configuration change")
	}
	if len(networks.savedNetworkDevices) != 0 {
		t.Fatalf("unchanged endpoint must not write membership, got %d writes", len(networks.savedNetworkDevices))
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
