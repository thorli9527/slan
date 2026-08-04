package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
	"github.com/slan/service-biz/internal/repository"
)

type deviceSessionTestDevices struct {
	networkRuntimeTestDevices
	savedSessions      []model.DeviceSession
	nextVirtualIP      int
	virtualIPConflicts int
	loseNextRotation   bool
}

func (s *deviceSessionTestDevices) SaveDevice(_ context.Context, device model.Device) error {
	if device.VirtualIP != "" && s.virtualIPConflicts > 0 {
		s.virtualIPConflicts--
		return repository.ErrDeviceVirtualIPConflict
	}
	if s.devices == nil {
		s.devices = map[string]model.Device{}
	}
	s.devices[device.DeviceID] = device
	return nil
}

func (s *deviceSessionTestDevices) SaveDeviceSession(_ context.Context, session model.DeviceSession) error {
	for index, current := range s.savedSessions {
		if current.SessionID == session.SessionID {
			s.savedSessions[index] = session
			return nil
		}
	}
	s.savedSessions = append(s.savedSessions, session)
	return nil
}

func (s *deviceSessionTestDevices) RotateDeviceSession(_ context.Context, currentRefreshToken string, next model.DeviceSession) (bool, error) {
	for index, session := range s.savedSessions {
		if session.DeviceID != next.DeviceID || session.RefreshToken != currentRefreshToken {
			continue
		}
		if s.loseNextRotation {
			s.loseNextRotation = false
			digest := sha256.Sum256([]byte(currentRefreshToken))
			winner := next
			winner.AccessToken = "access-winner"
			winner.RefreshToken = "refresh-winner"
			winner.PreviousRefreshTokenHash = hex.EncodeToString(digest[:])
			s.savedSessions[index] = winner
			return false, nil
		}
		s.savedSessions[index] = next
		return true, nil
	}
	return false, nil
}

func (s *deviceSessionTestDevices) DeleteDeviceSessionForRefreshReuse(_ context.Context, sessionID, previousRefreshTokenHash string, now int64) (bool, error) {
	for index, session := range s.savedSessions {
		if session.SessionID == sessionID && session.PreviousRefreshTokenHash == previousRefreshTokenHash && session.RefreshRotationGraceExpiry < now {
			s.savedSessions = append(s.savedSessions[:index], s.savedSessions[index+1:]...)
			return true, nil
		}
	}
	return false, nil
}

func (s *deviceSessionTestDevices) GetDeviceSessionByAccessToken(_ context.Context, token string) (model.DeviceSession, bool, error) {
	for _, session := range s.savedSessions {
		if session.AccessToken == token {
			return session, true, nil
		}
	}
	return model.DeviceSession{}, false, nil
}

func (s *deviceSessionTestDevices) GetDeviceSessionByRefreshToken(_ context.Context, token string) (model.DeviceSession, bool, error) {
	digest := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(digest[:])
	for _, session := range s.savedSessions {
		if session.RefreshToken == token || session.PreviousRefreshTokenHash == tokenHash {
			return session, true, nil
		}
	}
	return model.DeviceSession{}, false, nil
}

func (s *deviceSessionTestDevices) ListDeviceSessionsByDeviceID(_ context.Context, deviceID string) ([]model.DeviceSession, error) {
	items := make([]model.DeviceSession, 0, 1)
	for _, session := range s.savedSessions {
		if session.DeviceID == deviceID {
			items = append(items, session)
		}
	}
	return items, nil
}

func (s *deviceSessionTestDevices) DeleteDeviceSessionByAccessToken(_ context.Context, token string) error {
	items := s.savedSessions[:0]
	for _, session := range s.savedSessions {
		if session.AccessToken != token {
			items = append(items, session)
		}
	}
	s.savedSessions = items
	return nil
}

func (s *deviceSessionTestDevices) NewDeviceVirtualIPID() string {
	s.nextVirtualIP += 1
	return fmt.Sprintf("vip-%06d", s.nextVirtualIP)
}

type deviceSessionTestNetworks struct {
	networkRuntimeTestNetworks
}

func (s *deviceSessionTestNetworks) ListNetworksByOwner(context.Context, string) ([]model.Network, error) {
	out := []model.Network{}
	for _, item := range s.networks {
		out = append(out, item)
	}
	return out, nil
}

func (s *deviceSessionTestNetworks) SaveNetworkDevice(_ context.Context, item model.NetworkDevice) error {
	items := s.networkDevices[item.NetworkID]
	for index, current := range items {
		if current.DeviceID == item.DeviceID {
			items[index] = item
			s.networkDevices[item.NetworkID] = items
			return nil
		}
	}
	s.networkDevices[item.NetworkID] = append(items, item)
	return nil
}

func TestRenewDeviceSessionRetriesPreviousTokenIdempotently(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	devices := &deviceSessionTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{
			"device-1": {DeviceID: "device-1", Status: "active"},
		}},
		savedSessions: []model.DeviceSession{{
			SessionID: "device-session-1", DeviceID: "device-1", CredentialID: "dcred-1", AccessToken: "access-1",
			RefreshToken: "refresh-1", Status: tokenStatusActive, SessionMode: tokenModeLong,
			ExpiresAt: now.Add(time.Hour).Unix(), RefreshExpiry: now.Add(24 * time.Hour).Unix(),
		}},
	}
	service := DeviceSessionService{
		Devices: devices,
		Credentials: &deviceCredentialTestStore{items: map[string]model.DeviceCredential{
			"dcred-1": {CredentialID: "dcred-1", DeviceID: "device-1", Status: model.DeviceCredentialStatusActive, Scopes: deviceCredentialScope},
		}},
		Networks: &deviceSessionTestNetworks{networkRuntimeTestNetworks: networkRuntimeTestNetworks{
			networks: map[string]model.Network{}, networkDevices: map[string][]model.NetworkDevice{},
		}},
		MQTT:      mqttkit.DefaultConfig(),
		NewSessID: func(string) string { return "device-session-renewed" },
		Now:       func() time.Time { return now },
	}

	first, err := service.RenewDeviceSession(context.Background(), "access-1", RenewDeviceSessionInput{RefreshToken: "refresh-1"})
	if err != nil {
		t.Fatalf("first renewal: %v", err)
	}
	retry, err := service.RenewDeviceSession(context.Background(), "access-1", RenewDeviceSessionInput{RefreshToken: "refresh-1"})
	if err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	if retry.Session.AccessToken != first.Session.AccessToken || retry.Session.RefreshToken != first.Session.RefreshToken {
		t.Fatalf("retry rotated device tokens again: first=%#v retry=%#v", first.Session, retry.Session)
	}
	if first.Session.CredentialID != "dcred-1" || retry.Session.CredentialID != "dcred-1" {
		t.Fatalf("renewal lost authorization key binding: first=%#v retry=%#v", first.Session, retry.Session)
	}
	if first.Profile.GlobalIP != "10.0.1.1" || retry.Profile.GlobalIP != first.Profile.GlobalIP {
		t.Fatalf("renewal must repair and preserve device IP: first=%q retry=%q", first.Profile.GlobalIP, retry.Profile.GlobalIP)
	}
	if devices.nextVirtualIP != 1 {
		t.Fatalf("renewal must allocate missing IP exactly once, got %d", devices.nextVirtualIP)
	}
}

func TestRenewDeviceSessionRevokesSessionOnExpiredRefreshTokenReuse(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	oldRefreshToken := "refresh-consumed"
	digest := sha256.Sum256([]byte(oldRefreshToken))
	devices := &deviceSessionTestDevices{savedSessions: []model.DeviceSession{{
		SessionID: "device-session-rotated", DeviceID: "device-1", CredentialID: "dcred-1",
		AccessToken: "access-current", RefreshToken: "refresh-current", Status: tokenStatusActive,
		PreviousRefreshTokenHash: hex.EncodeToString(digest[:]), RefreshRotationGraceExpiry: now.Add(-time.Second).Unix(),
		ExpiresAt: now.Add(time.Hour).Unix(), RefreshExpiry: now.Add(24 * time.Hour).Unix(),
	}}}
	audit := &deviceCredentialTestAudit{}
	service := DeviceSessionService{Devices: devices, Audit: audit, Now: func() time.Time { return now }}

	_, err := service.RenewDeviceSession(context.Background(), "access-current", RenewDeviceSessionInput{
		RefreshToken: oldRefreshToken, RemoteIP: "198.51.100.20",
	})
	if err != ErrUnauthorized {
		t.Fatalf("expired refresh token reuse error = %v, want %v", err, ErrUnauthorized)
	}
	if len(devices.savedSessions) != 0 {
		t.Fatalf("refresh token reuse retained session: %#v", devices.savedSessions)
	}
	if len(audit.events) != 1 || audit.events[0].Action != "refresh_token_reuse" || audit.events[0].Status != "warning" ||
		audit.events[0].ResourceID != "device-session-rotated" || audit.events[0].RemoteIP != "198.51.100.20" {
		t.Fatalf("unexpected refresh token reuse audit: %#v", audit.events)
	}
}

func TestRenewDeviceSessionReturnsWinningConcurrentRotation(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	devices := &deviceSessionTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{
			"device-1": {DeviceID: "device-1", Status: "active", VirtualIP: "10.0.1.1"},
		}},
		loseNextRotation: true,
		savedSessions: []model.DeviceSession{{
			SessionID: "device-session-1", DeviceID: "device-1", CredentialID: "dcred-1", AccessToken: "access-1",
			RefreshToken: "refresh-1", Status: tokenStatusActive, SessionMode: tokenModeLong,
			ExpiresAt: now.Add(time.Hour).Unix(), RefreshExpiry: now.Add(24 * time.Hour).Unix(),
		}},
	}
	service := DeviceSessionService{
		Devices: devices,
		Credentials: &deviceCredentialTestStore{items: map[string]model.DeviceCredential{
			"dcred-1": {CredentialID: "dcred-1", DeviceID: "device-1", Status: model.DeviceCredentialStatusActive, Scopes: deviceCredentialScope},
		}},
		Networks: &deviceSessionTestNetworks{networkRuntimeTestNetworks: networkRuntimeTestNetworks{
			networks: map[string]model.Network{}, networkDevices: map[string][]model.NetworkDevice{},
		}},
		MQTT: mqttkit.DefaultConfig(), NewSessID: func(string) string { return "device-session-loser" }, Now: func() time.Time { return now },
	}

	view, err := service.RenewDeviceSession(context.Background(), "access-1", RenewDeviceSessionInput{RefreshToken: "refresh-1"})
	if err != nil {
		t.Fatalf("concurrent renewal retry: %v", err)
	}
	if view.Session.AccessToken != "access-winner" || view.Session.RefreshToken != "refresh-winner" {
		t.Fatalf("renewal did not return winning rotation: %#v", view.Session)
	}
}

func TestDeviceSessionRejectsInactiveDevice(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	devices := &deviceSessionTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{
			"device-1": {DeviceID: "device-1", Status: "disabled"},
		}},
		savedSessions: []model.DeviceSession{{
			SessionID: "device-session-1", DeviceID: "device-1", CredentialID: "dcred-1",
			AccessToken: "access-1", Status: tokenStatusActive, ExpiresAt: now.Add(time.Hour).Unix(),
		}},
	}
	credentials := &deviceCredentialTestStore{items: map[string]model.DeviceCredential{
		"dcred-1": {CredentialID: "dcred-1", DeviceID: "device-1", Status: model.DeviceCredentialStatusActive, Scopes: deviceCredentialScope},
	}}
	service := DeviceSessionService{Devices: devices, Credentials: credentials, Now: func() time.Time { return now }}
	if _, err := service.AuthenticateDeviceSession(context.Background(), "access-1"); err != ErrUnauthorized {
		t.Fatalf("inactive device authentication error = %v, want %v", err, ErrUnauthorized)
	}
}

func TestRenewDeviceSessionRemovesRotatedSessionWhenCredentialIsConcurrentlyRevoked(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	devices := &deviceSessionTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{
			"device-1": {DeviceID: "device-1", Status: "active", VirtualIP: "10.0.1.1"},
		}},
		savedSessions: []model.DeviceSession{{
			SessionID: "device-session-1", DeviceID: "device-1", CredentialID: "dcred-1",
			AccessToken: "access-1", RefreshToken: "refresh-1", Status: tokenStatusActive, SessionMode: tokenModeLong,
			ExpiresAt: now.Add(time.Hour).Unix(), RefreshExpiry: now.Add(24 * time.Hour).Unix(),
		}},
	}
	credentials := &deviceCredentialTestStore{
		revokeOnMark: true,
		items: map[string]model.DeviceCredential{
			"dcred-1": {CredentialID: "dcred-1", DeviceID: "device-1", Status: model.DeviceCredentialStatusActive, Scopes: deviceCredentialScope},
		},
	}
	service := DeviceSessionService{
		Devices: devices, Credentials: credentials,
		Networks: &deviceSessionTestNetworks{networkRuntimeTestNetworks: networkRuntimeTestNetworks{
			networks: map[string]model.Network{}, networkDevices: map[string][]model.NetworkDevice{},
		}},
		MQTT: mqttkit.DefaultConfig(), NewSessID: func(string) string { return "device-session-renewed" },
		Now: func() time.Time { return now },
	}

	if _, err := service.RenewDeviceSession(context.Background(), "access-1", RenewDeviceSessionInput{RefreshToken: "refresh-1"}); err != ErrUnauthorized {
		t.Fatalf("concurrent revocation renewal error = %v, want %v", err, ErrUnauthorized)
	}
	if len(devices.savedSessions) != 0 {
		t.Fatalf("concurrent revocation left sessions behind: %#v", devices.savedSessions)
	}
}
