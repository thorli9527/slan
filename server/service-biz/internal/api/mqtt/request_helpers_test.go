package mqtt

import (
	"net/http/httptest"
	"testing"
)

func TestTopicFromValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		values map[string]any
		want   string
	}{
		{
			name:   "topic direct",
			values: map[string]any{"topic": " slan/test "},
			want:   "slan/test",
		},
		{
			name:   "topic filter direct",
			values: map[string]any{"topicFilter": " slan/filter "},
			want:   "slan/filter",
		},
		{
			name: "topic nested in client info",
			values: map[string]any{
				"clientInfo": map[string]any{"topic": " nested/topic "},
			},
			want: "nested/topic",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := topicFromValues(tt.values); got != tt.want {
				t.Fatalf("topicFromValues() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCheckRequestInput(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "/mqtt/bifromq/check", nil)
	req.Header.Set("deviceId", "header-device")
	req.Header.Set("userId", "header-user")

	input := checkRequest{
		Values: map[string]any{
			"principal": "device:demo",
			"sub": map[string]any{
				"topicFilter": "slan/networks/demo/broadcast",
			},
		},
	}.input(req)

	if input.Principal != "device:demo" {
		t.Fatalf("principal = %q", input.Principal)
	}
	if input.DeviceID != "header-device" {
		t.Fatalf("deviceId = %q", input.DeviceID)
	}
	if input.UserID != "header-user" {
		t.Fatalf("userId = %q", input.UserID)
	}
	if input.Topic != "slan/networks/demo/broadcast" {
		t.Fatalf("topic = %q", input.Topic)
	}
	if !input.Subscribe {
		t.Fatal("expected subscribe to be true")
	}
	if input.Connect {
		t.Fatal("expected connect to be false")
	}
}

func TestCheckRequestInputReadsClientIdentityVariants(t *testing.T) {
	t.Parallel()

	req := httptest.NewRequest("POST", "/mqtt/bifromq/check", nil)
	req.Header.Set("clientID", "header-client")
	req.Header.Set("userName", "device:header-device:9999999999")

	input := checkRequest{
		Values: map[string]any{
			"clientInfo": map[string]any{
				"clientID": " conn-client ",
				"userName": " device:conn-device:9999999999 ",
			},
			"sub": map[string]any{
				"topicFilter": " slan/devices/conn-device/control/down ",
			},
		},
	}.input(req)

	if input.ClientID != "conn-client" {
		t.Fatalf("clientId = %q", input.ClientID)
	}
	if input.Username != "device:conn-device:9999999999" {
		t.Fatalf("username = %q", input.Username)
	}
	if input.Topic != "slan/devices/conn-device/control/down" {
		t.Fatalf("topic = %q", input.Topic)
	}
	if !input.Subscribe {
		t.Fatal("expected subscribe to be true")
	}
	if input.Connect {
		t.Fatal("expected connect to be false")
	}
}

func TestEndpointReportRequestInput(t *testing.T) {
	t.Parallel()

	input := endpointReportRequest{
		Values: map[string]any{
			"payload": map[string]any{
				"networkId": "net-1",
				"deviceId":  "dev-1",
				"nodeId":    "node-1",
				"natType":   "easy",
				"endpoints": []any{
					map[string]any{
						"type":      "udp",
						"address":   "1.2.3.4:5000",
						"updatedAt": float64(123),
					},
				},
			},
		},
	}.input()

	if input.NetworkID != "net-1" || input.DeviceID != "dev-1" || input.NodeID != "node-1" {
		t.Fatalf("unexpected ids: %#v", input)
	}
	if input.NATType != "easy" {
		t.Fatalf("natType = %q", input.NATType)
	}
	if len(input.Endpoints) != 1 {
		t.Fatalf("endpoint count = %d", len(input.Endpoints))
	}
	if input.Endpoints[0].UpdatedAt != 123 {
		t.Fatalf("updatedAt = %d", input.Endpoints[0].UpdatedAt)
	}
}

func TestPathHealthReportRequestInput(t *testing.T) {
	t.Parallel()

	input := pathHealthReportRequest{
		Values: map[string]any{
			"payload": map[string]any{
				"networkId":       "net-1",
				"deviceId":        "dev-1",
				"peerNodeId":      "peer-1",
				"pathType":        "relay_udp",
				"activePath":      "relay",
				"relayTransport":  "udp",
				"endpoint":        "relay.example:443",
				"derpNodeId":      "derp-1",
				"observedRttMs":   float64(18),
				"packetLossPpm":   float64(22),
				"pathScore":       float64(90),
				"relayMtu":        float64(1280),
				"maxFramePayload": float64(1200),
				"ticketExpiresAt": "2026-07-03T12:00:00Z",
				"ticketRenewDue":  true,
				"pathDowngrades":  float64(1),
				"pathUpgrades":    float64(2),
				"lastPathChange":  "relay_udp",
				"sampledAtMs":     float64(456),
			},
		},
	}.input()

	if input.NetworkID != "net-1" || input.DeviceID != "dev-1" || input.PeerNodeID != "peer-1" {
		t.Fatalf("unexpected ids: %#v", input)
	}
	if input.ObservedRttMs != 18 || input.PacketLossPpm != 22 || input.PathScore != 90 {
		t.Fatalf("unexpected path stats: %#v", input)
	}
	if input.RelayMtu != 1280 || input.MaxFramePayload != 1200 {
		t.Fatalf("unexpected relay sizing: %#v", input)
	}
	if !input.TicketRenewDue {
		t.Fatal("expected ticket renew due")
	}
}
