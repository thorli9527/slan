package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

type deviceCredentialTestStore struct {
	items                         map[string]model.DeviceCredential
	revokeOnMark                  bool
	enforceActiveDeviceUniqueness bool
	cleanupCutoff                 int64
}

type deviceCredentialTestAudit struct {
	events []model.AuditEvent
}

func (a *deviceCredentialTestAudit) ListAuditEvents(context.Context, int) ([]model.AuditEvent, error) {
	return a.events, nil
}

func (a *deviceCredentialTestAudit) SaveAuditEvent(_ context.Context, event model.AuditEvent) error {
	a.events = append(a.events, event)
	return nil
}

func (s *deviceCredentialTestStore) ListDeviceCredentials(_ context.Context, deviceID string) ([]model.DeviceCredential, error) {
	items := make([]model.DeviceCredential, 0, len(s.items))
	for _, item := range s.items {
		if deviceID == "" || item.DeviceID == deviceID {
			items = append(items, item)
		}
	}
	return items, nil
}

func (s *deviceCredentialTestStore) GetDeviceCredential(_ context.Context, credentialID string) (model.DeviceCredential, bool, error) {
	item, ok := s.items[credentialID]
	return item, ok, nil
}

func (s *deviceCredentialTestStore) GetDeviceCredentialByKeyID(_ context.Context, keyID string) (model.DeviceCredential, bool, error) {
	for _, item := range s.items {
		if item.KeyID == keyID {
			return item, true, nil
		}
	}
	return model.DeviceCredential{}, false, nil
}

func (s *deviceCredentialTestStore) SaveDeviceCredential(_ context.Context, item model.DeviceCredential) error {
	if s.items == nil {
		s.items = map[string]model.DeviceCredential{}
	}
	s.items[item.CredentialID] = item
	return nil
}

func (s *deviceCredentialTestStore) BindDeviceCredential(_ context.Context, credentialID, deviceID string, updatedAt int64) (bool, error) {
	item, ok := s.items[credentialID]
	if !ok || item.DeviceID != "" || item.Status != model.DeviceCredentialStatusActive {
		return false, nil
	}
	if s.enforceActiveDeviceUniqueness {
		for id, existing := range s.items {
			if id != credentialID && existing.DeviceID == deviceID && existing.Status == model.DeviceCredentialStatusActive {
				return false, errors.New("duplicate active device credential")
			}
		}
	}
	item.DeviceID, item.UpdatedAt = deviceID, updatedAt
	s.items[credentialID] = item
	return true, nil
}

func (s *deviceCredentialTestStore) MarkDeviceCredentialUsed(_ context.Context, credentialID, deviceID string, now int64, remoteIP string) (bool, error) {
	item, ok := s.items[credentialID]
	if s.revokeOnMark && ok {
		item.Status, item.RevokedAt, item.UpdatedAt = model.DeviceCredentialStatusRevoked, now, now
		s.items[credentialID] = item
	}
	if !ok || item.DeviceID != deviceID || item.Status != model.DeviceCredentialStatusActive {
		return false, nil
	}
	item.LastUsedAt, item.LastUsedIP, item.UpdatedAt = now, remoteIP, now
	s.items[credentialID] = item
	return true, nil
}

func TestDeviceCredentialExchangeRemovesSessionWhenCredentialIsConcurrentlyRevoked(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	devices := &deviceSessionTestDevices{networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{
		"device-1": {DeviceID: "device-1", Name: "gateway", Status: "active"},
	}}}
	credentials := &deviceCredentialTestStore{revokeOnMark: true}
	service := DeviceCredentialService{
		Devices: devices, Credentials: credentials,
		Networks: &deviceSessionTestNetworks{networkRuntimeTestNetworks: networkRuntimeTestNetworks{
			networks: map[string]model.Network{}, networkDevices: map[string][]model.NetworkDevice{},
		}},
		MQTT: mqttkit.DefaultConfig(), Pepper: "test-pepper",
		NewSessID: func(scope string) string { return scope + "-1" }, Now: func() time.Time { return now },
	}

	created, err := service.CreateDeviceCredential(context.Background(), CreateDeviceCredentialInput{DeviceID: "device-1", Name: "installer"})
	if err != nil {
		t.Fatalf("CreateDeviceCredential returned error: %v", err)
	}
	if _, err := service.ExchangeDeviceCredential(context.Background(), ExchangeDeviceCredentialInput{Key: created.Key}); err != ErrUnauthorized {
		t.Fatalf("expected concurrent revocation to reject exchange, got %v", err)
	}
	if len(devices.savedSessions) != 0 {
		t.Fatalf("expected rejected exchange to remove its session, got %d", len(devices.savedSessions))
	}
}

func (s *deviceCredentialTestStore) RevokeDeviceCredential(_ context.Context, credentialID string, now int64) (bool, error) {
	item, ok := s.items[credentialID]
	if !ok {
		return false, nil
	}
	item.Status, item.RevokedAt, item.UpdatedAt = model.DeviceCredentialStatusRevoked, now, now
	s.items[credentialID] = item
	return true, nil
}

func (s *deviceCredentialTestStore) DeleteInvalidDeviceCredentialsBefore(_ context.Context, cutoff int64) (int64, error) {
	s.cleanupCutoff = cutoff
	var deleted int64
	for credentialID, item := range s.items {
		if item.Status != model.DeviceCredentialStatusActive && item.UpdatedAt <= cutoff {
			delete(s.items, credentialID)
			deleted++
		}
	}
	return deleted, nil
}

func TestCleanupInvalidDeviceCredentialsKeepsActiveAndRecentlyRevokedKeys(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	store := &deviceCredentialTestStore{items: map[string]model.DeviceCredential{
		"old-revoked": {
			CredentialID: "old-revoked", Status: model.DeviceCredentialStatusRevoked,
			UpdatedAt: now.Add(-8 * 24 * time.Hour).Unix(),
		},
		"recent-revoked": {
			CredentialID: "recent-revoked", Status: model.DeviceCredentialStatusRevoked,
			UpdatedAt: now.Add(-6 * 24 * time.Hour).Unix(),
		},
		"old-active": {
			CredentialID: "old-active", Status: model.DeviceCredentialStatusActive,
			UpdatedAt: now.Add(-30 * 24 * time.Hour).Unix(),
		},
	}}
	service := DeviceCredentialService{Credentials: store, Now: func() time.Time { return now }}

	deleted, err := service.CleanupInvalidDeviceCredentials(context.Background())
	if err != nil {
		t.Fatalf("CleanupInvalidDeviceCredentials returned error: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("deleted credentials = %d, want 1", deleted)
	}
	if store.cleanupCutoff != now.Add(-7*24*time.Hour).Unix() {
		t.Fatalf("cleanup cutoff = %d, want %d", store.cleanupCutoff, now.Add(-7*24*time.Hour).Unix())
	}
	if _, ok := store.items["old-revoked"]; ok {
		t.Fatal("old revoked credential was not deleted")
	}
	if _, ok := store.items["recent-revoked"]; !ok {
		t.Fatal("recently revoked credential was deleted")
	}
	if _, ok := store.items["old-active"]; !ok {
		t.Fatal("active credential was deleted")
	}
}

func TestDeviceCredentialPepperRotationAcceptsOnlyConfiguredKeyring(t *testing.T) {
	secret := "authorization-key-secret"
	storedHash := credentialSecretHash("old-pepper", secret)
	if !credentialSecretMatches(storedHash, secret, "new-pepper", []string{"old-pepper"}) {
		t.Fatal("configured previous pepper did not validate existing authorization key")
	}
	if credentialSecretMatches(storedHash, secret, "new-pepper", nil) {
		t.Fatal("removed previous pepper still validated authorization key")
	}
	if credentialSecretMatches(storedHash, "wrong-secret", "new-pepper", []string{"old-pepper"}) {
		t.Fatal("wrong authorization key secret matched pepper keyring")
	}
}

func TestDeviceCredentialLifecycle(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	devices := &deviceSessionTestDevices{networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{
		"device-1": {DeviceID: "device-1", Name: "gateway", Status: "active"},
	}}}
	credentials := &deviceCredentialTestStore{}
	audit := &deviceCredentialTestAudit{}
	service := DeviceCredentialService{
		Devices: devices, Credentials: credentials, Audit: audit,
		Networks: &deviceSessionTestNetworks{networkRuntimeTestNetworks: networkRuntimeTestNetworks{
			networks: map[string]model.Network{}, networkDevices: map[string][]model.NetworkDevice{},
		}},
		MQTT: mqttkit.DefaultConfig(), Pepper: "test-pepper",
		NewSessID: func(scope string) string { return scope + "-1" }, Now: func() time.Time { return now },
	}

	created, err := service.CreateDeviceCredential(context.Background(), CreateDeviceCredentialInput{DeviceID: "device-1", Name: "installer"})
	if err != nil {
		t.Fatalf("CreateDeviceCredential returned error: %v", err)
	}
	if strings.HasPrefix(created.Key, deviceCredentialLegacyKeyPrefix) {
		t.Fatalf("credential key still contains legacy prefix: %q", created.Key)
	}
	if got, want := len(created.Key), 41; got != want {
		t.Fatalf("credential key length = %d, want %d: %q", got, want, created.Key)
	}
	keyID, secret, ok := parseDeviceCredentialKey(created.Key)
	if !ok || len(keyID) != 16 || len(secret) != 24 {
		t.Fatalf("unexpected compact credential key parts: keyID=%q secret=%q", keyID, secret)
	}
	legacyKeyID, legacySecret, legacyOK := parseDeviceCredentialKey(deviceCredentialLegacyKeyPrefix + created.Key)
	if !legacyOK || legacyKeyID != keyID || legacySecret != secret {
		t.Fatalf("legacy credential key is no longer parseable")
	}
	stored := credentials.items[created.CredentialID]
	if stored.SecretHash == "" || strings.Contains(created.Key, stored.SecretHash) || stored.SecretHash == created.Key {
		t.Fatalf("credential secret must only be stored as a non-reversible digest")
	}

	view, err := service.ExchangeDeviceCredential(context.Background(), ExchangeDeviceCredentialInput{Key: created.Key, DeviceID: "device-1", RemoteIP: "127.0.0.1"})
	if err != nil {
		t.Fatalf("ExchangeDeviceCredential returned error: %v", err)
	}
	if view.Session.CredentialID != created.CredentialID || len(devices.savedSessions) != 1 {
		t.Fatalf("expected session to reference credential, got %+v", view.Session)
	}
	if _, err := service.ExchangeDeviceCredential(context.Background(), ExchangeDeviceCredentialInput{
		Key: created.Key, DeviceID: "device-1", RemoteIP: "198.51.100.20",
	}); err != nil {
		t.Fatalf("ExchangeDeviceCredential from changed source returned error: %v", err)
	}
	if _, err := service.ExchangeDeviceCredential(context.Background(), ExchangeDeviceCredentialInput{Key: created.Key, DeviceID: "device-2"}); err != ErrUnauthorized {
		t.Fatalf("expected cross-device exchange to be unauthorized, got %v", err)
	}

	revoked, err := service.RevokeDeviceCredential(context.Background(), created.CredentialID)
	if err != nil {
		t.Fatalf("RevokeDeviceCredential returned error: %v", err)
	}
	if revoked.Status != model.DeviceCredentialStatusRevoked || len(devices.savedSessions) != 0 {
		t.Fatalf("expected revoked credential and removed sessions, got %+v sessions=%d", revoked, len(devices.savedSessions))
	}
	if _, err := service.ExchangeDeviceCredential(context.Background(), ExchangeDeviceCredentialInput{Key: created.Key}); err != ErrUnauthorized {
		t.Fatalf("expected revoked key exchange to be unauthorized, got %v", err)
	}
	if len(audit.events) != 7 || audit.events[0].Action != "create" || audit.events[1].Action != "exchange" ||
		audit.events[2].Action != "exchange_source_changed" || audit.events[2].Status != "warning" ||
		audit.events[2].RemoteIP != "198.51.100.20" || !strings.Contains(audit.events[2].Detail, "127.0.0.1") ||
		audit.events[3].Action != "exchange" || audit.events[5].Action != "revoke" || audit.events[6].Status != "failure" {
		t.Fatalf("unexpected credential audit events: %#v", audit.events)
	}
}

func TestCreateDeviceCredentialRevokesPreviousActiveCredentialForDevice(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	devices := &deviceSessionTestDevices{networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{
		"device-1": {DeviceID: "device-1", Name: "gateway", Status: "active"},
	}}}
	credentials := &deviceCredentialTestStore{}
	service := DeviceCredentialService{
		Devices: devices, Credentials: credentials, Pepper: "test-pepper",
		Now: func() time.Time { return now },
	}

	first, err := service.CreateDeviceCredential(context.Background(), CreateDeviceCredentialInput{
		DeviceID: "device-1", Name: "first key",
	})
	if err != nil {
		t.Fatalf("first CreateDeviceCredential returned error: %v", err)
	}
	second, err := service.CreateDeviceCredential(context.Background(), CreateDeviceCredentialInput{
		DeviceID: "device-1", Name: "replacement key",
	})
	if err != nil {
		t.Fatalf("second CreateDeviceCredential returned error: %v", err)
	}

	if got := credentials.items[first.CredentialID].Status; got != model.DeviceCredentialStatusRevoked {
		t.Fatalf("previous credential status = %q, want revoked", got)
	}
	if got := credentials.items[second.CredentialID].Status; got != model.DeviceCredentialStatusActive {
		t.Fatalf("replacement credential status = %q, want active", got)
	}
	assertSingleActiveDeviceCredential(t, credentials.items, "device-1", second.CredentialID)
}

func TestUnboundDeviceCredentialBindingRevokesPreviousActiveCredentialForDevice(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	devices := &deviceSessionTestDevices{networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{
		"device-1": {DeviceID: "device-1", Name: "gateway", Status: "active", VirtualIP: "10.0.1.1"},
	}}}
	credentials := &deviceCredentialTestStore{enforceActiveDeviceUniqueness: true}
	service := DeviceCredentialService{
		Devices: devices, Credentials: credentials,
		Networks: &deviceSessionTestNetworks{networkRuntimeTestNetworks: networkRuntimeTestNetworks{
			networks: map[string]model.Network{}, networkDevices: map[string][]model.NetworkDevice{},
		}},
		MQTT: mqttkit.DefaultConfig(), Pepper: "test-pepper",
		NewSessID: func(scope string) string { return scope + "-1" }, Now: func() time.Time { return now },
	}

	previous, err := service.CreateDeviceCredential(context.Background(), CreateDeviceCredentialInput{
		DeviceID: "device-1", Name: "previous key",
	})
	if err != nil {
		t.Fatalf("bound CreateDeviceCredential returned error: %v", err)
	}
	replacement, err := service.CreateDeviceCredential(context.Background(), CreateDeviceCredentialInput{Name: "replacement key"})
	if err != nil {
		t.Fatalf("unbound CreateDeviceCredential returned error: %v", err)
	}
	if _, err := service.ExchangeDeviceCredential(context.Background(), ExchangeDeviceCredentialInput{
		Key: replacement.Key, DeviceID: "device-1",
	}); err != nil {
		t.Fatalf("ExchangeDeviceCredential returned error: %v", err)
	}

	if got := credentials.items[previous.CredentialID].Status; got != model.DeviceCredentialStatusRevoked {
		t.Fatalf("previous credential status = %q, want revoked", got)
	}
	assertSingleActiveDeviceCredential(t, credentials.items, "device-1", replacement.CredentialID)
}

func assertSingleActiveDeviceCredential(t *testing.T, items map[string]model.DeviceCredential, deviceID, credentialID string) {
	t.Helper()
	activeIDs := make([]string, 0, 1)
	for _, item := range items {
		if item.DeviceID == deviceID && item.Status == model.DeviceCredentialStatusActive {
			activeIDs = append(activeIDs, item.CredentialID)
		}
	}
	if len(activeIDs) != 1 || activeIDs[0] != credentialID {
		t.Fatalf("active credentials for %s = %v, want [%s]", deviceID, activeIDs, credentialID)
	}
}

func TestCreateDeviceCredentialRejectsUnsupportedScope(t *testing.T) {
	service := DeviceCredentialService{
		Credentials: &deviceCredentialTestStore{}, Pepper: "test-pepper",
		Now: func() time.Time { return time.Unix(1_700_000_000, 0) },
	}
	if _, err := service.CreateDeviceCredential(context.Background(), CreateDeviceCredentialInput{
		Name: "misleading limited key", Scopes: "read_only_device",
	}); err != ErrInvalidArgument {
		t.Fatalf("unsupported scope error = %v, want %v", err, ErrInvalidArgument)
	}
}

func TestDeviceCredentialAuditUsesAuthenticatedOperator(t *testing.T) {
	audit := &deviceCredentialTestAudit{}
	service := DeviceCredentialService{Audit: audit}
	ctx := WithAuthenticatedOperator(context.Background(), "operator-1")
	service.recordCredentialAudit(ctx, "create", "dcred-1", "success", "203.0.113.10", 1_700_000_000)
	if len(audit.events) != 1 || audit.events[0].ActorType != "operator" || audit.events[0].ActorID != "operator-1" {
		t.Fatalf("unexpected operator audit event: %#v", audit.events)
	}
	if audit.events[0].RemoteIP != "203.0.113.10" {
		t.Fatalf("operator audit remote IP = %q", audit.events[0].RemoteIP)
	}
}

func TestUnboundDeviceCredentialBindsAndCreatesDeviceOnFirstExchange(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	devices := &deviceSessionTestDevices{networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{}}}
	credentials := &deviceCredentialTestStore{}
	service := DeviceCredentialService{
		Devices: devices, Credentials: credentials,
		Networks: &deviceSessionTestNetworks{networkRuntimeTestNetworks: networkRuntimeTestNetworks{
			networks: map[string]model.Network{}, networkDevices: map[string][]model.NetworkDevice{},
		}},
		MQTT: mqttkit.DefaultConfig(), Pepper: "test-pepper",
		NewSessID: func(scope string) string { return scope + "-1" }, Now: func() time.Time { return now },
	}

	created, err := service.CreateDeviceCredential(context.Background(), CreateDeviceCredentialInput{Name: "unattended installer"})
	if err != nil {
		t.Fatalf("CreateDeviceCredential returned error: %v", err)
	}
	view, err := service.ExchangeDeviceCredential(context.Background(), ExchangeDeviceCredentialInput{
		Key: created.Key, DeviceID: "device-new", Platform: "macos", DeviceVersion: "0.1.0",
	})
	if err != nil {
		t.Fatalf("ExchangeDeviceCredential returned error: %v", err)
	}
	if view.Profile.Device.DeviceID != "device-new" || credentials.items[created.CredentialID].DeviceID != "device-new" {
		t.Fatalf("expected first exchange to bind device-new, got view=%+v credential=%+v", view.Profile, credentials.items[created.CredentialID])
	}
	if device := devices.devices["device-new"]; device.DeviceID != "device-new" || device.Status != "active" ||
		device.VirtualIP != "10.0.1.1" || device.Platform != "macos" || device.DeviceVersion != "0.1.0" {
		t.Fatalf("expected managed device creation, got %+v", device)
	}
	if view.Profile.GlobalIP != "10.0.1.1" {
		t.Fatalf("expected first exchange to return allocated virtual IP, got %+v", view.Profile.Device)
	}
	if _, err := service.ExchangeDeviceCredential(context.Background(), ExchangeDeviceCredentialInput{Key: created.Key, DeviceID: "device-other"}); err != ErrUnauthorized {
		t.Fatalf("expected rebound exchange to be unauthorized, got %v", err)
	}
}

func TestDeviceCredentialExchangeUpdatesExistingDeviceClientMetadata(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	devices := &deviceSessionTestDevices{networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{
		"device-1": {
			DeviceID: "device-1", Name: "gateway", Platform: "unknown",
			VirtualIP: "10.0.1.9", Status: "active",
		},
	}}}
	credentials := &deviceCredentialTestStore{}
	service := DeviceCredentialService{
		Devices: devices, Credentials: credentials,
		Networks: &deviceSessionTestNetworks{networkRuntimeTestNetworks: networkRuntimeTestNetworks{
			networks: map[string]model.Network{}, networkDevices: map[string][]model.NetworkDevice{},
		}},
		MQTT: mqttkit.DefaultConfig(), Pepper: "test-pepper",
		NewSessID: func(scope string) string { return scope + "-1" }, Now: func() time.Time { return now },
	}

	created, err := service.CreateDeviceCredential(context.Background(), CreateDeviceCredentialInput{
		DeviceID: "device-1", Name: "installer",
	})
	if err != nil {
		t.Fatalf("CreateDeviceCredential returned error: %v", err)
	}
	if _, err := service.ExchangeDeviceCredential(context.Background(), ExchangeDeviceCredentialInput{
		Key: created.Key, DeviceID: "device-1", Platform: "MacOS", DeviceVersion: "0.1.0",
	}); err != nil {
		t.Fatalf("ExchangeDeviceCredential returned error: %v", err)
	}
	device := devices.devices["device-1"]
	if device.Platform != "macos" || device.DeviceVersion != "0.1.0" {
		t.Fatalf("existing device client metadata was not updated: %+v", device)
	}
}

func TestDeviceCredentialFirstExchangeRetriesVirtualIPConflict(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	devices := &deviceSessionTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{}},
		virtualIPConflicts:        1,
	}
	credentials := &deviceCredentialTestStore{}
	service := DeviceCredentialService{
		Devices: devices, Credentials: credentials,
		Networks: &deviceSessionTestNetworks{networkRuntimeTestNetworks: networkRuntimeTestNetworks{
			networks: map[string]model.Network{}, networkDevices: map[string][]model.NetworkDevice{},
		}},
		MQTT: mqttkit.DefaultConfig(), Pepper: "test-pepper",
		NewSessID: func(scope string) string { return scope + "-1" }, Now: func() time.Time { return now },
	}

	created, err := service.CreateDeviceCredential(context.Background(), CreateDeviceCredentialInput{Name: "installer"})
	if err != nil {
		t.Fatalf("CreateDeviceCredential returned error: %v", err)
	}
	view, err := service.ExchangeDeviceCredential(context.Background(), ExchangeDeviceCredentialInput{
		Key: created.Key, DeviceID: "device-retry",
	})
	if err != nil {
		t.Fatalf("ExchangeDeviceCredential returned error: %v", err)
	}
	if view.Profile.GlobalIP != "10.0.1.2" || devices.nextVirtualIP != 2 {
		t.Fatalf("expected conflict retry to allocate 10.0.1.2, got profile=%+v allocations=%d", view.Profile, devices.nextVirtualIP)
	}
}

func TestDeviceCredentialCreatesPermanentAuthorizationKey(t *testing.T) {
	createdAt := time.Unix(1_700_000_000, 0)
	now := createdAt
	devices := &deviceSessionTestDevices{networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{
		"device-1": {DeviceID: "device-1", Name: "gateway", Status: "active"},
	}}}
	credentials := &deviceCredentialTestStore{}
	service := DeviceCredentialService{
		Devices: devices, Credentials: credentials,
		Networks: &deviceSessionTestNetworks{networkRuntimeTestNetworks: networkRuntimeTestNetworks{
			networks: map[string]model.Network{}, networkDevices: map[string][]model.NetworkDevice{},
		}},
		MQTT: mqttkit.DefaultConfig(), Pepper: "test-pepper",
		NewSessID: func(scope string) string { return scope + "-1" }, Now: func() time.Time { return now },
	}

	created, err := service.CreateDeviceCredential(context.Background(), CreateDeviceCredentialInput{
		DeviceID: "device-1", Name: "permanent-key",
	})
	if err != nil {
		t.Fatalf("CreateDeviceCredential returned error: %v", err)
	}
	now = createdAt.Add(10 * 365 * 24 * time.Hour)
	view, err := service.ExchangeDeviceCredential(context.Background(), ExchangeDeviceCredentialInput{Key: created.Key})
	if err != nil {
		t.Fatalf("ExchangeDeviceCredential returned error: %v", err)
	}
	if view.Session.ExpiresAt <= now.Unix() || view.Session.RefreshExpiry <= view.Session.ExpiresAt {
		t.Fatalf("authorization key unexpectedly shortened the device session: %#v", view.Session)
	}
}
