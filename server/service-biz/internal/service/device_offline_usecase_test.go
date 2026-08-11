package service

import (
	"context"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
)

func newDeviceOfflineTestRig(now time.Time) (DeviceOfflineService, *deviceSessionTestDevices, *deviceCredentialTestStore, *deviceCredentialTestAudit, *networkAccessTestDevicePublisher) {
	devices := &deviceSessionTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{
			"device-1": {DeviceID: "device-1", Name: "gateway", Status: "disabled"},
		}},
		savedSessions: []model.DeviceSession{{SessionID: "session-1", DeviceID: "device-1", AccessToken: "access-1", CredentialID: "dcred-1"}},
	}
	credentials := &deviceCredentialTestStore{items: map[string]model.DeviceCredential{
		"dcred-1": {
			CredentialID: "dcred-1", DeviceID: "device-1", Status: model.DeviceCredentialStatusActive,
			DisableNotifiedAt: now.Unix(),
		},
	}}
	audit := &deviceCredentialTestAudit{}
	publisher := &networkAccessTestDevicePublisher{}
	service := DeviceOfflineService{
		Devices: devices, Credentials: credentials, Audit: audit, Publisher: publisher,
		Now: func() time.Time { return now },
	}
	return service, devices, credentials, audit, publisher
}

func TestReportDeviceOfflineAcksAndRevokesCredential(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	service, devices, credentials, audit, _ := newDeviceOfflineTestRig(now)

	if err := service.ReportDeviceOffline(context.Background(), "device-1", "dcred-1"); err != nil {
		t.Fatal(err)
	}
	credential := credentials.items["dcred-1"]
	if credential.OfflineAckAt != now.Unix() {
		t.Fatalf("offline ack not recorded: %+v", credential)
	}
	if credential.Status != model.DeviceCredentialStatusRevoked || credential.RevokedAt != now.Unix() {
		t.Fatalf("credential not revoked: %+v", credential)
	}
	if len(devices.savedSessions) != 0 {
		t.Fatalf("device session kept after offline report: %+v", devices.savedSessions)
	}
	if len(audit.events) != 1 || audit.events[0].Action != "offline_report" || audit.events[0].ActorType != "device" {
		t.Fatalf("unexpected offline report audit: %#v", audit.events)
	}
}

func TestReportDeviceOfflineIsIdempotent(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	service, _, credentials, audit, _ := newDeviceOfflineTestRig(now)

	if err := service.ReportDeviceOffline(context.Background(), "device-1", "dcred-1"); err != nil {
		t.Fatal(err)
	}
	if err := service.ReportDeviceOffline(context.Background(), "device-1", "dcred-1"); err != nil {
		t.Fatalf("second offline report failed: %v", err)
	}
	if credential := credentials.items["dcred-1"]; credential.Status != model.DeviceCredentialStatusRevoked {
		t.Fatalf("credential status = %q", credential.Status)
	}
	if len(audit.events) != 2 {
		t.Fatalf("audit events = %d, want 2", len(audit.events))
	}
}

func TestRetryPendingDeviceOfflineRepublishesNotification(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	service, _, credentials, _, publisher := newDeviceOfflineTestRig(now)
	item := credentials.items["dcred-1"]
	item.DisableNotifiedAt = now.Add(-time.Hour).Unix()
	credentials.items["dcred-1"] = item

	result, err := service.RetryPendingDeviceOffline(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 1 || result.Republished != 1 || result.Revoked != 0 {
		t.Fatalf("retry result = %+v", result)
	}
	if len(publisher.events) != 1 || publisher.events[0].Type != "device_disabled" || publisher.deviceIDs[0] != "device-1" {
		t.Fatalf("expected republished device_disabled event, got ids=%v events=%#v", publisher.deviceIDs, publisher.events)
	}
	credential := credentials.items["dcred-1"]
	if credential.DisableNotifiedAt != now.Unix() || credential.DisableNotifyCount != 1 {
		t.Fatalf("notify tracking not updated: %+v", credential)
	}
	if credential.Status != model.DeviceCredentialStatusActive {
		t.Fatalf("credential revoked before fallback deadline: %+v", credential)
	}
}

func TestRetryPendingDeviceOfflineForceRevokesAfterDeadline(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	service, devices, credentials, audit, publisher := newDeviceOfflineTestRig(now)
	item := credentials.items["dcred-1"]
	item.DisableNotifiedAt = now.Add(-25 * time.Hour).Unix()
	credentials.items["dcred-1"] = item

	result, err := service.RetryPendingDeviceOffline(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 1 || result.Revoked != 1 || result.Republished != 0 {
		t.Fatalf("retry result = %+v", result)
	}
	if len(publisher.events) != 0 {
		t.Fatalf("fallback revoke must not republish: %#v", publisher.events)
	}
	credential := credentials.items["dcred-1"]
	if credential.Status != model.DeviceCredentialStatusRevoked {
		t.Fatalf("credential not force revoked: %+v", credential)
	}
	if len(devices.savedSessions) != 0 {
		t.Fatalf("device session kept after fallback revoke: %+v", devices.savedSessions)
	}
	if len(audit.events) != 1 || audit.events[0].Action != "offline_fallback_revoke" {
		t.Fatalf("unexpected fallback audit: %#v", audit.events)
	}
}

func TestRetryPendingDeviceOfflineSkipsReenabledDevice(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	service, devices, credentials, _, publisher := newDeviceOfflineTestRig(now)
	device := devices.devices["device-1"]
	device.Status = "active"
	devices.devices["device-1"] = device

	result, err := service.RetryPendingDeviceOffline(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Scanned != 1 || result.Republished != 0 || result.Revoked != 0 {
		t.Fatalf("retry result = %+v", result)
	}
	if len(publisher.events) != 0 {
		t.Fatalf("reenabled device must not be notified: %#v", publisher.events)
	}
	if credential := credentials.items["dcred-1"]; credential.DisableNotifyCount != 0 {
		t.Fatalf("notify count changed for reenabled device: %+v", credential)
	}
}
