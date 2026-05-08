package mqttauth

import (
	"testing"
	"time"

	"github.com/slan/server/server-biz/configs"
)

func TestRelayCredentialCanPublishHeartbeatOnly(t *testing.T) {
	cfg := configs.DefaultConfig().MQTT
	cfg.Enabled = true
	cfg.TopicPrefix = "slan/devices"
	cfg.UsernamePrefix = "slan"
	cfg.PasswordSecret = "test-mqtt-secret"

	credential := RelayCredential(cfg, "relay-cn-local-udp", time.Now())
	if credential == nil {
		t.Fatal("expected relay credential")
	}
	result, ok := ValidateCredential(cfg, credential.ClientID, credential.Username, credential.Password)
	if !ok || !result.Allow || result.Principal != "relay" || result.DeviceID != "relay-cn-local-udp" {
		t.Fatalf("unexpected relay auth result: %#v ok=%v", result, ok)
	}
	if !AllowTopicAccess(cfg, result.Principal, result.DeviceID, RelayHeartbeatTopic(cfg, result.DeviceID), false) {
		t.Fatal("expected relay heartbeat publish to be allowed")
	}
	if AllowTopicAccess(cfg, result.Principal, result.DeviceID, ControlUpTopic(cfg, "dev-1"), false) {
		t.Fatal("expected relay control publish to be denied")
	}
}

func TestServerSubscriberSuffixClientIDIsAccepted(t *testing.T) {
	cfg := configs.DefaultConfig().MQTT
	cfg.Enabled = true
	cfg.UsernamePrefix = "slan"
	cfg.PasswordSecret = "test-mqtt-secret"

	credential := ServerSubscriberCredential(cfg, time.Now())
	if credential == nil {
		t.Fatal("expected server credential")
	}
	if _, ok := ValidateCredential(cfg, credential.ClientID+"-relay-heartbeat", credential.Username, credential.Password); !ok {
		t.Fatal("expected suffixed server subscriber client id to validate")
	}
	for _, suffix := range []string{"-control-up", "-network-state", "-control-down-msg-1", "-auth-callback-cb-1"} {
		if _, ok := ValidateCredential(cfg, credential.ClientID+suffix, credential.Username, credential.Password); !ok {
			t.Fatalf("expected suffixed server client id %s to validate", suffix)
		}
	}
}

func TestDeviceCredentialCanPublishControlTelemetry(t *testing.T) {
	cfg := configs.DefaultConfig().MQTT
	cfg.Enabled = true
	cfg.TopicPrefix = "slan/devices"
	cfg.UsernamePrefix = "slan"
	cfg.PasswordSecret = "test-mqtt-secret"

	credential := DeviceCredential(cfg, "dev-1", "dev-1", time.Now())
	if credential == nil {
		t.Fatal("expected device credential")
	}
	result, ok := ValidateCredential(cfg, credential.ClientID, credential.Username, credential.Password)
	if !ok || !result.Allow || result.Principal != "device" || result.DeviceID != "dev-1" {
		t.Fatalf("unexpected device auth result: %#v ok=%v", result, ok)
	}
	allowedTopics := []string{
		"slan/devices/dev-1/heartbeat",
		"slan/devices/dev-1/runtime-state",
		"slan/devices/dev-1/control/up",
		"slan/devices/dev-1/control/ack",
		"slan/devices/dev-1/networks/net-1/state",
	}
	for _, topic := range allowedTopics {
		if !AllowTopicAccess(cfg, result.Principal, result.DeviceID, topic, false) {
			t.Fatalf("expected device publish to be allowed for %s", topic)
		}
	}
	if AllowTopicAccess(cfg, result.Principal, result.DeviceID, "slan/devices/dev-2/heartbeat", false) {
		t.Fatal("expected another device heartbeat publish to be denied")
	}
}

func TestNetworkBroadcastTopicAccess(t *testing.T) {
	cfg := configs.DefaultConfig().MQTT
	cfg.Enabled = true
	cfg.TopicPrefix = "slan/devices"
	cfg.UsernamePrefix = "slan"
	cfg.PasswordSecret = "test-mqtt-secret"

	device := DeviceCredential(cfg, "dev-1", "dev-1", time.Now())
	if device == nil {
		t.Fatal("expected device credential")
	}
	deviceAuth, ok := ValidateCredential(cfg, device.ClientID, device.Username, device.Password)
	if !ok {
		t.Fatal("expected device credential to validate")
	}
	topic := NetworkBroadcastTopic(cfg, "net-1")
	if !AllowTopicAccess(cfg, deviceAuth.Principal, deviceAuth.DeviceID, topic, true) {
		t.Fatal("expected device subscribe to network broadcast to be allowed")
	}
	if AllowTopicAccess(cfg, deviceAuth.Principal, deviceAuth.DeviceID, topic, false) {
		t.Fatal("expected device publish to network broadcast to be denied")
	}

	server := ServerSubscriberCredential(cfg, time.Now())
	if server == nil {
		t.Fatal("expected server credential")
	}
	serverAuth, ok := ValidateCredential(cfg, server.ClientID+"-network-broadcast", server.Username, server.Password)
	if !ok {
		t.Fatal("expected server credential to validate")
	}
	if !AllowTopicAccess(cfg, serverAuth.Principal, serverAuth.DeviceID, topic, false) {
		t.Fatal("expected server publish to network broadcast to be allowed")
	}
}
