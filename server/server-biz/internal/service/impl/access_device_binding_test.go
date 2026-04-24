package impl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/service"
	"github.com/slan/server/server-biz/internal/util"
)

func TestLogin_RejectsDeviceBoundToAnotherUser(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()

	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-1",
		Email:        "user1@example.com",
		PasswordHash: util.HashPassword("Password-1"),
	}); err != nil {
		t.Fatalf("create user-1: %v", err)
	}
	if err := state.pg.CreateUser(ctx, repo.User{
		UserID:       "user-2",
		Email:        "user2@example.com",
		PasswordHash: util.HashPassword("Password-2"),
	}); err != nil {
		t.Fatalf("create user-2: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-2",
		UserID:    "user-2",
		MachineID: "machine-2",
		Name:      "device-2",
		Platform:  "windows",
		Status:    "online",
		CreatedAt: time.Now().Unix(),
		PublicKey: nil,
	}); err != nil {
		t.Fatalf("insert device: %v", err)
	}

	auth := dbAuthService{state: state}
	_, err := auth.Login(dto.LoginRequest{
		Email:    "user1@example.com",
		Password: "Password-1",
		DeviceID: "dev-2",
	})
	if !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestAuthenticate_InvalidatesExpiredDeviceBoundSession(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()

	tokenStore := state.tokens.(*memoryTokenStore)
	tokenStore.accessTokens["access-1"] = repo.AccessTokenSession{
		UserID:   "user-1",
		DeviceID: "dev-1",
		IssuedAt: time.Now().Add(-3 * time.Minute).Unix(),
	}

	userID, err := dbTokenVerifier{state: state}.Authenticate("access-1")
	if !errors.Is(err, service.ErrUnauthorized) {
		t.Fatalf("expected unauthorized, got user=%q err=%v", userID, err)
	}
	if _, err := tokenStore.Authenticate(ctx, "access-1"); err == nil {
		t.Fatalf("expected access token to be invalidated")
	}
}

func TestAuthenticate_AllowsFreshDeviceBoundSession(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()

	tokenStore := state.tokens.(*memoryTokenStore)
	tokenStore.accessTokens["access-1"] = repo.AccessTokenSession{
		UserID:   "user-1",
		DeviceID: "dev-1",
		IssuedAt: time.Now().Add(-5 * time.Minute).Unix(),
	}

	if err := state.pg.CreateControlSession(ctx, repo.ControlSession{
		ControlSessionID: "ctrl-1",
		UserID:           "user-1",
		DeviceID:         "dev-1",
		NodeID:           "node-1",
		NetworkID:        "net-1",
		SessionToken:     "session-1",
		ConnectedAt:      time.Now().Add(-5 * time.Minute).Unix(),
		LastSeenAt:       time.Now().Add(-30 * time.Second).Unix(),
	}); err != nil {
		t.Fatalf("create control session: %v", err)
	}

	userID, err := dbTokenVerifier{state: state}.Authenticate("access-1")
	if err != nil {
		t.Fatalf("authenticate fresh device-bound session: %v", err)
	}
	if userID != "user-1" {
		t.Fatalf("expected user-1, got %s", userID)
	}
}

func TestAuthenticate_AllowsSessionWithoutDeviceBinding(t *testing.T) {
	state := newNetworkTestState(t)
	tokenStore := state.tokens.(*memoryTokenStore)
	tokenStore.accessTokens["access-1"] = repo.AccessTokenSession{
		UserID:   "user-1",
		IssuedAt: time.Now().Add(-24 * time.Hour).Unix(),
	}

	userID, err := dbTokenVerifier{state: state}.Authenticate("access-1")
	if err != nil {
		t.Fatalf("authenticate unbound session: %v", err)
	}
	if userID != "user-1" {
		t.Fatalf("expected user-1, got %s", userID)
	}
}
