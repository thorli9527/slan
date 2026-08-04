package ops

import (
	"testing"

	servicepkg "github.com/slan/service-biz/internal/service"
)

func TestManagedDevicePayloadIncludesClientVersion(t *testing.T) {
	payload := managedDevicePayload(servicepkg.OpsManagedDeviceView{
		Device: servicepkg.DeviceView{
			DeviceID:      "device-1",
			DeviceVersion: "2.4.1",
		},
	})

	if got := payload["deviceVersion"]; got != "2.4.1" {
		t.Fatalf("deviceVersion = %v, want 2.4.1", got)
	}
}
