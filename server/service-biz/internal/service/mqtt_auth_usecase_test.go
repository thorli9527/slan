package service

import (
	"context"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

func TestMQTTAuthenticateRequiresActiveBoundDeviceCredential(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	cfg := mqttkit.DefaultConfig()
	credentials := &deviceCredentialTestStore{items: map[string]model.DeviceCredential{
		"dcred-1": {
			CredentialID: "dcred-1", DeviceID: "device-1", Status: model.DeviceCredentialStatusActive,
			Scopes: deviceCredentialScope,
		},
	}}
	devices := &deviceSessionTestDevices{networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{
		"device-1": {DeviceID: "device-1", Status: "active"},
	}}}
	service := MQTTWebhookService{Devices: devices, Credentials: credentials, Config: cfg, Now: func() time.Time { return now }}
	credential := mqttkit.CredentialForDevice(cfg, "device-1", "dcred-1", now, now.Add(time.Hour).Unix())
	input := MQTTAuthInput{ClientID: credential.ClientID, Username: credential.Username, Password: credential.Password}

	view, err := service.Authenticate(context.Background(), input)
	if err != nil || !view.Allowed || view.DeviceID != "device-1" {
		t.Fatalf("active credential authentication: view=%+v err=%v", view, err)
	}
	item := credentials.items["dcred-1"]
	item.Status = model.DeviceCredentialStatusRevoked
	credentials.items["dcred-1"] = item
	view, err = service.Authenticate(context.Background(), input)
	if err != nil || view.Allowed {
		t.Fatalf("revoked credential authentication: view=%+v err=%v", view, err)
	}
	item.Status = model.DeviceCredentialStatusActive
	credentials.items["dcred-1"] = item
	device := devices.devices["device-1"]
	device.Status = "disabled"
	devices.devices["device-1"] = device
	view, err = service.Authenticate(context.Background(), input)
	if err != nil || view.Allowed {
		t.Fatalf("disabled device authentication: view=%+v err=%v", view, err)
	}
}
