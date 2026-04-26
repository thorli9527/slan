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
	response, ok := ValidateCredential(cfg.MQTT, credential.ClientID, credential.Username, credential.Password)
	if !ok || !response.Allow || response.Principal != "device" || response.DeviceID != "dev-1" {
		t.Fatalf("unexpected auth response: %+v ok=%v", response, ok)
	}
}

func TestValidateCredentialServerSubscriber(t *testing.T) {
	cfg := configs.DefaultConfig()
	cfg.MQTT.Enabled = true
	credential := ServerSubscriberCredential(cfg.MQTT, time.Unix(100, 0))
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
