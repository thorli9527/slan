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
}
