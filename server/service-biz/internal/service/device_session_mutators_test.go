package service

import (
	"testing"

	"github.com/slan/service-biz/internal/model"
)

func TestApplyRenewDeviceSessionInputDoesNotChangeAdministrativeStatus(t *testing.T) {
	networkEnabled := false
	device, updated := applyRenewDeviceSessionInput(
		model.Device{DeviceID: "device-1", Status: "active", LastSeenAt: 100},
		RenewDeviceSessionInput{NetworkEnabled: &networkEnabled, LastSeenAt: 100},
		200,
	)

	if updated {
		t.Fatal("network runtime state unexpectedly updated the managed device")
	}
	if device.Status != "active" {
		t.Fatalf("device status = %q, want active", device.Status)
	}
}

func TestApplyUpdateDeviceRuntimeDoesNotChangeAdministrativeStatus(t *testing.T) {
	device := applyUpdateDeviceRuntime(
		model.Device{DeviceID: "device-1", Status: "active", LastSeenAt: 100},
		UpdateDeviceRuntimeInput{Status: "inactive", ReportedAtMS: 200_000},
		200,
	)

	if device.Status != "active" {
		t.Fatalf("device status = %q, want active", device.Status)
	}
	if device.LastSeenAt != 200 {
		t.Fatalf("device last seen = %d, want 200", device.LastSeenAt)
	}
}
