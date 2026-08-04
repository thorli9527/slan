package app

import (
	"testing"

	servicepkg "github.com/slan/service-biz/internal/service"
)

func TestDNSRecordPayloadResolvesTargetDeviceVirtualIP(t *testing.T) {
	items := []servicepkg.DNSRecordView{{
		RecordID:       "record-1",
		ZoneID:         "zone-1",
		Name:           "android-a",
		Type:           "A",
		TargetDeviceID: "device-a",
	}}

	payloads := dnsRecordPayloads(
		items,
		map[string]string{"zone-1": "dual.lan"},
		map[string]string{"10.0.1.1": "device-a"},
		map[string]string{"device-a": "10.0.1.1"},
	)

	if got := payloads[0]["targetIp"]; got != "10.0.1.1" {
		t.Fatalf("targetIp = %v, want 10.0.1.1", got)
	}
	if got := payloads[0]["fqdn"]; got != "android-a.dual.lan" {
		t.Fatalf("fqdn = %v, want android-a.dual.lan", got)
	}
}
