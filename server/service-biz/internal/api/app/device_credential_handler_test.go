package app

import (
	"encoding/json"
	"testing"

	servicepkg "github.com/slan/service-biz/internal/service"
)

func TestDeviceCredentialExchangeDevicePayloadIncludesAssignedIP(t *testing.T) {
	profile := servicepkg.DeviceProfileView{
		Device: servicepkg.DeviceView{
			DeviceID: "device-1",
			Status:   "active",
		},
		ActiveNetworkID:  "network-1",
		MembershipStatus: "active",
		CurrentVirtualIP: "10.0.1.63",
		VirtualIP:        "10.0.1.63",
		GlobalIP:         "10.0.1.63",
	}

	payload := appControlDeviceProfilePayload(profile)
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal device payload: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode device payload: %v", err)
	}
	for _, field := range []string{"currentVirtualIp", "virtualIp", "globalIp"} {
		if decoded[field] != "10.0.1.63" {
			t.Fatalf("expected %s in activation payload, got %#v", field, decoded[field])
		}
	}
	if decoded["activeNetworkId"] != "network-1" {
		t.Fatalf("expected active network in activation payload, got %#v", decoded["activeNetworkId"])
	}
}
