package mqttauth

import (
	"testing"
	"time"

	"github.com/slan/server/server-biz/configs"
)

func TestValidateCredentialDevice(t *testing.T) {
	cfg := configs.DefaultConfig()
	cfg.MQTT.Enabled = true
	credential := DeviceCredential(cfg.MQTT, "dev-1", "machine-1", time.Unix(100, 0))
	if credential.ClientID != "slan-dev-1" {
		t.Fatalf("unexpected bifromq-compatible client id: %s", credential.ClientID)
	}
	response, ok := ValidateCredential(cfg.MQTT, credential.ClientID, credential.Username, credential.Password)
	if !ok || !response.Allow || response.Principal != "device" || response.DeviceID != "dev-1" {
		t.Fatalf("unexpected auth response: %+v ok=%v", response, ok)
	}
}

func TestValidateCredentialServerSubscriber(t *testing.T) {
	cfg := configs.DefaultConfig()
	cfg.MQTT.Enabled = true
	credential := ServerSubscriberCredential(cfg.MQTT, time.Unix(100, 0))
	if credential.ClientID != "slan-server" {
		t.Fatalf("unexpected bifromq-compatible server client id: %s", credential.ClientID)
	}
	response, ok := ValidateCredential(cfg.MQTT, credential.ClientID, credential.Username, credential.Password)
	if !ok || !response.Allow || response.Principal != "server" || response.DeviceID != "" {
		t.Fatalf("unexpected auth response: %+v ok=%v", response, ok)
	}
}

func TestValidateCredentialRejectsServerAsDevice(t *testing.T) {
	cfg := configs.DefaultConfig()
	cfg.MQTT.Enabled = true
	credential := ServerSubscriberCredential(cfg.MQTT, time.Unix(100, 0))
	if deviceID, ok := ValidateDevice(cfg.MQTT, credential.ClientID, credential.Username, credential.Password); ok || deviceID != "" {
		t.Fatalf("server subscriber must not validate as device, got device=%s ok=%v", deviceID, ok)
	}
}

func TestAllowTopicAccess(t *testing.T) {
	cfg := configs.DefaultConfig()
	if !AllowTopicAccess(cfg.MQTT, "device", "dev-1", "slan/devices/dev-1/networks/net-1/state", false) {
		t.Fatal("expected device publish to own topic to be allowed")
	}
	if AllowTopicAccess(cfg.MQTT, "device", "dev-1", "slan/devices/dev-2/networks/net-1/state", false) {
		t.Fatal("expected device publish to another device topic to be denied")
	}
	if AllowTopicAccess(cfg.MQTT, "device", "dev-1", "slan/devices/dev-1/anything", false) {
		t.Fatal("expected device publish outside network state topics to be denied")
	}
	if AllowTopicAccess(cfg.MQTT, "device", "dev-1", "slan/devices/dev-1/networks//state", false) {
		t.Fatal("expected device publish with empty network id to be denied")
	}
	if !AllowTopicAccess(cfg.MQTT, "device", "dev-1", "slan/devices/dev-1/#", true) {
		t.Fatal("expected device subscribe to own topic prefix to be allowed")
	}
	if !AllowTopicAccess(cfg.MQTT, "server", "", "slan/devices/+/networks/+/state", true) {
		t.Fatal("expected server state subscription to be allowed")
	}
	if AllowTopicAccess(cfg.MQTT, "server", "", "slan/devices/#", true) {
		t.Fatal("expected broad server subscription to be denied")
	}
}
