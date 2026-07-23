package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/pkg/mqttkit"
)

type deviceSessionTestUsers struct {
	users map[string]model.User
}

func (s *deviceSessionTestUsers) ListUsers(context.Context) ([]model.User, error) { return nil, nil }

func (s *deviceSessionTestUsers) GetUser(_ context.Context, userID string) (model.User, bool, error) {
	item, ok := s.users[userID]
	return item, ok, nil
}

func (s *deviceSessionTestUsers) GetByEmail(_ context.Context, email string) (model.User, bool, error) {
	for _, item := range s.users {
		if item.Email == email {
			return item, true, nil
		}
	}
	return model.User{}, false, nil
}

func (s *deviceSessionTestUsers) SaveUser(_ context.Context, user model.User) error {
	if s.users == nil {
		s.users = map[string]model.User{}
	}
	s.users[user.UserID] = user
	return nil
}

type deviceSessionTestDevices struct {
	networkRuntimeTestDevices
	savedSessions []model.DeviceSession
	nextVirtualIP int
}

func (s *deviceSessionTestDevices) SaveDevice(_ context.Context, device model.Device) error {
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

func (s *deviceSessionTestNetworks) ListNetworksByOwner(_ context.Context, ownerID string) ([]model.Network, error) {
	out := []model.Network{}
	for _, item := range s.networks {
		if item.OwnerID == ownerID {
			out = append(out, item)
		}
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

func TestBindDeviceSessionAutoRegistersMissingDevice(t *testing.T) {
	now := time.Unix(1700000000, 0)
	devices := &deviceSessionTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{
			devices: map[string]model.Device{},
		},
	}
	networks := &deviceSessionTestNetworks{
		networkRuntimeTestNetworks: networkRuntimeTestNetworks{
			networks: map[string]model.Network{
				"net-1": {
					NetworkID: "net-1",
					OwnerID:   "user-1",
					Name:      "Default",
					CIDR:      "10.0.0.0/24",
					Default:   true,
					Status:    "active",
				},
			},
			networkDevices: map[string][]model.NetworkDevice{},
		},
	}
	service := DeviceSessionService{
		Users: &deviceSessionTestUsers{
			users: map[string]model.User{
				"user-1": {UserID: "user-1", Email: "user-1@example.test", Status: "active"},
			},
		},
		Devices:   devices,
		Networks:  networks,
		MQTT:      mqttkit.DefaultConfig(),
		NewSessID: func(scope string) string { return scope + "-1" },
		Now:       func() time.Time { return now },
	}

	view, err := service.BindDeviceSession(context.Background(), BindDeviceSessionInput{
		UserID:        "user-1",
		DeviceID:      "android-1",
		Name:          "Pixel 3a",
		Platform:      "android",
		Alias:         "android-phone",
		OSName:        "android",
		OSVersion:     "14",
		PublicKey:     "pub-1",
		DeviceVersion: "1.0.0",
		CountryCode:   "CN",
	})
	if err != nil {
		t.Fatalf("BindDeviceSession returned error: %v", err)
	}
	device, ok := devices.devices["android-1"]
	if !ok {
		t.Fatalf("expected device to be auto-registered")
	}
	if device.OwnerID != "user-1" {
		t.Fatalf("expected device owner user-1, got %s", device.OwnerID)
	}
	if len(devices.savedSessions) != 1 {
		t.Fatalf("expected one saved device session, got %d", len(devices.savedSessions))
	}
	if len(networks.networkDevices["net-1"]) != 0 {
		t.Fatalf("expected bind not to create individual network membership, got %+v", networks.networkDevices["net-1"])
	}
	if view.Profile.Device.DeviceID != "android-1" {
		t.Fatalf("expected view device android-1, got %+v", view.Profile.Device)
	}
	if view.Profile.OwnerEmail != "user-1@example.test" {
		t.Fatalf("expected owner email to be populated, got %s", view.Profile.OwnerEmail)
	}
	if view.Session.DeviceID != "android-1" || view.Session.AccessToken == "" {
		t.Fatalf("expected bound session to be returned, got %+v", view.Session)
	}
}

func TestBindDeviceSessionReplacesPreviousDeviceToken(t *testing.T) {
	now := time.Unix(1700000000, 0)
	devices := &deviceSessionTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{
			devices: map[string]model.Device{
				"mac-1": {
					DeviceID: "mac-1",
					OwnerID:  "user-1",
					Name:     "Mac",
					Platform: "macos",
					Status:   "active",
				},
			},
		},
	}
	networks := &deviceSessionTestNetworks{
		networkRuntimeTestNetworks: networkRuntimeTestNetworks{
			networks: map[string]model.Network{
				"net-1": {
					NetworkID: "net-1",
					OwnerID:   "user-1",
					Name:      "Default",
					CIDR:      "10.0.0.0/24",
					Default:   true,
					Status:    "active",
				},
			},
			networkDevices: map[string][]model.NetworkDevice{},
		},
	}
	nextSession := 0
	service := DeviceSessionService{
		Users: &deviceSessionTestUsers{users: map[string]model.User{
			"user-1": {UserID: "user-1", Email: "user-1@example.test", Status: "active"},
		}},
		Devices:  devices,
		Networks: networks,
		MQTT:     mqttkit.DefaultConfig(),
		NewSessID: func(scope string) string {
			nextSession++
			return fmt.Sprintf("%s-%d", scope, nextSession)
		},
		Now: func() time.Time { return now },
	}

	first, err := service.BindDeviceSession(context.Background(), BindDeviceSessionInput{
		UserID: "user-1", DeviceID: "mac-1",
	})
	if err != nil {
		t.Fatalf("first BindDeviceSession returned error: %v", err)
	}
	second, err := service.BindDeviceSession(context.Background(), BindDeviceSessionInput{
		UserID: "user-1", DeviceID: "mac-1",
	})
	if err != nil {
		t.Fatalf("second BindDeviceSession returned error: %v", err)
	}
	if first.Session.AccessToken == second.Session.AccessToken {
		t.Fatal("expected replacement access token")
	}
	items, err := devices.ListDeviceSessionsByDeviceID(context.Background(), "mac-1")
	if err != nil {
		t.Fatalf("ListDeviceSessionsByDeviceID returned error: %v", err)
	}
	if len(items) != 1 || items[0].AccessToken != second.Session.AccessToken {
		t.Fatalf("expected only replacement session, got %+v", items)
	}
	if _, ok, _ := devices.GetDeviceSessionByAccessToken(context.Background(), first.Session.AccessToken); ok {
		t.Fatal("expected previous access token to be invalid")
	}
	if _, err := service.AuthenticateDeviceSession(context.Background(), first.Session.AccessToken); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected previous access token to be unauthorized, got %v", err)
	}
	authenticated, err := service.AuthenticateDeviceSession(context.Background(), second.Session.AccessToken)
	if err != nil {
		t.Fatalf("AuthenticateDeviceSession returned error for replacement token: %v", err)
	}
	if authenticated.DeviceID != "mac-1" {
		t.Fatalf("expected authenticated device mac-1, got %q", authenticated.DeviceID)
	}
}

func TestRenewDeviceSessionRetriesPreviousTokenIdempotently(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	devices := &deviceSessionTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{devices: map[string]model.Device{
			"device-1": {DeviceID: "device-1", OwnerID: "user-1", Status: "active"},
		}},
		savedSessions: []model.DeviceSession{{
			SessionID: "device-session-1", DeviceID: "device-1", AccessToken: "access-1",
			RefreshToken: "refresh-1", Status: tokenStatusActive, SessionMode: tokenModeLong,
			ExpiresAt: now.Add(time.Hour).Unix(), RefreshExpiry: now.Add(24 * time.Hour).Unix(),
		}},
	}
	service := DeviceSessionService{
		Users: &deviceSessionTestUsers{users: map[string]model.User{
			"user-1": {UserID: "user-1", Email: "user@example.test", Status: "active"},
		}},
		Devices: devices,
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
}

func TestBindDeviceSessionMigratesLegacyIPAndIgnoresInactiveNetworkMembership(t *testing.T) {
	now := time.Unix(1700000000, 0)
	devices := &deviceSessionTestDevices{
		networkRuntimeTestDevices: networkRuntimeTestDevices{
			devices: map[string]model.Device{
				"linux-1": {
					DeviceID:  "linux-1",
					OwnerID:   "user-1",
					VirtualIP: "172.16.0.1",
					Name:      "Docker Linux",
					Platform:  "linux",
					Status:    "active",
					CreatedAt: now.Unix(),
					UpdatedAt: now.Unix(),
				},
			},
		},
	}
	networks := &deviceSessionTestNetworks{
		networkRuntimeTestNetworks: networkRuntimeTestNetworks{
			networks: map[string]model.Network{
				"net-1": {
					NetworkID: "net-1",
					OwnerID:   "user-1",
					Name:      "Default",
					CIDR:      "10.0.0.0/24",
					Default:   true,
					Status:    "active",
				},
			},
			networkDevices: map[string][]model.NetworkDevice{
				"net-1": {{
					NetworkID:    "net-1",
					DeviceID:     "linux-1",
					Enabled:      false,
					MemberStatus: model.NetworkMemberStatusRemoved,
				}},
			},
		},
	}
	service := DeviceSessionService{
		Users: &deviceSessionTestUsers{
			users: map[string]model.User{
				"user-1": {UserID: "user-1", Email: "user-1@example.test", Status: "active"},
			},
		},
		Devices:   devices,
		Networks:  networks,
		MQTT:      mqttkit.DefaultConfig(),
		NewSessID: func(scope string) string { return scope + "-1" },
		Now:       func() time.Time { return now },
	}

	view, err := service.BindDeviceSession(context.Background(), BindDeviceSessionInput{
		UserID:    "user-1",
		DeviceID:  "linux-1",
		Name:      "Docker Linux",
		Platform:  "linux",
		Alias:     "docker-linux",
		PublicKey: "pub-1",
	})
	if err != nil {
		t.Fatalf("BindDeviceSession returned error: %v", err)
	}
	if view.Profile.ActiveNetworkID != "" || len(view.MQTT.NetworkIDs) != 0 {
		t.Fatalf("expected inactive membership to be excluded from profile and MQTT topics, got profile=%+v mqtt=%+v", view.Profile, view.MQTT)
	}
	if view.Profile.Device.DeviceID != "linux-1" {
		t.Fatalf("expected existing device to be preserved, got %+v", view.Profile.Device)
	}
	if got := devices.devices["linux-1"].VirtualIP; got != "10.0.1.1" {
		t.Fatalf("expected legacy virtual IP to migrate to 10.0.1.1, got %q", got)
	}
}
