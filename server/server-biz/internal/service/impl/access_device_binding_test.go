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

func TestRefresh_RotatesRefreshToken(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()

	if err := state.tokens.StoreRefreshToken(ctx, "refresh-old", "user-1", time.Hour); err != nil {
		t.Fatalf("store refresh token: %v", err)
	}

	auth := dbAuthService{state: state}
	refreshed, err := auth.Refresh(dto.RefreshTokenRequest{RefreshToken: "refresh-old"})
	if err != nil {
		t.Fatalf("refresh token: %v", err)
	}
	if refreshed.UserID != "user-1" || refreshed.AccessToken == "" || refreshed.RefreshToken == "" {
		t.Fatalf("expected new auth response, got %+v", refreshed)
	}
	if _, err := state.tokens.AuthenticateRefreshToken(ctx, "refresh-old"); err == nil {
		t.Fatal("expected old refresh token to be revoked")
	}
	if _, err := state.tokens.AuthenticateRefreshToken(ctx, refreshed.RefreshToken); err != nil {
		t.Fatalf("expected new refresh token to be valid: %v", err)
	}
}

func TestRefresh_RejectsRefreshTokenReplay(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()
	if err := state.tokens.StoreRefreshToken(ctx, "refresh-old", "user-1", time.Hour); err != nil {
		t.Fatalf("store refresh token: %v", err)
	}

	auth := dbAuthService{state: state}
	if _, err := auth.Refresh(dto.RefreshTokenRequest{RefreshToken: "refresh-old"}); err != nil {
		t.Fatalf("first refresh token use: %v", err)
	}
	_, err := auth.Refresh(dto.RefreshTokenRequest{RefreshToken: "refresh-old"})
	if !errors.Is(err, service.ErrUnauthorized) {
		t.Fatalf("expected unauthorized replay, got %v", err)
	}
}

func TestRefresh_RejectsDeviceBoundToAnotherUser(t *testing.T) {
	state := newNetworkTestState(t)
	ctx := context.Background()

	if err := state.tokens.StoreRefreshToken(ctx, "refresh-1", "user-1", time.Hour); err != nil {
		t.Fatalf("store refresh token: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-2",
		UserID:    "user-2",
		MachineID: "machine-2",
		Name:      "device-2",
		Platform:  "windows",
		Status:    "online",
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("insert device: %v", err)
	}

	_, err := dbAuthService{state: state}.Refresh(dto.RefreshTokenRequest{
		RefreshToken: "refresh-1",
		DeviceID:     "dev-2",
	})
	if !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestConsoleLoginKey_IsSingleUseAndDeviceBound(t *testing.T) {
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
		DeviceID:  "dev-1",
		UserID:    "user-1",
		MachineID: "machine-1",
		Name:      "device-1",
		Platform:  "windows",
		Status:    "online",
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("insert dev-1: %v", err)
	}
	if err := state.pg.InsertDevice(ctx, repo.Device{
		DeviceID:  "dev-2",
		UserID:    "user-2",
		MachineID: "machine-2",
		Name:      "device-2",
		Platform:  "windows",
		Status:    "online",
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("insert dev-2: %v", err)
	}

	auth := dbAuthService{state: state}
	if _, err := auth.CreateConsoleLoginKey("user-1", dto.CreateConsoleLoginKeyRequest{DeviceID: "dev-2"}); !errors.Is(err, service.ErrForbidden) {
		t.Fatalf("expected forbidden for foreign device, got %v", err)
	}
	key, err := auth.CreateConsoleLoginKey("user-1", dto.CreateConsoleLoginKeyRequest{DeviceID: "dev-1"})
	if err != nil {
		t.Fatalf("create console login key: %v", err)
	}
	if key.LoginKey == "" || key.ExpiresIn <= 0 {
		t.Fatalf("expected login key with expiry, got %+v", key)
	}
	resp, err := auth.ConsumeConsoleLoginKey(dto.ConsumeConsoleLoginKeyRequest{LoginKey: key.LoginKey})
	if err != nil {
		t.Fatalf("consume console login key: %v", err)
	}
	if resp.UserID != "user-1" || resp.AccessToken == "" || resp.RefreshToken == "" {
		t.Fatalf("expected auth response for user-1, got %+v", resp)
	}
	if session, err := state.tokens.Authenticate(ctx, resp.AccessToken); err != nil || session.DeviceID != "dev-1" {
		t.Fatalf("expected device-bound access token, session=%+v err=%v", session, err)
	}
	if _, err := auth.ConsumeConsoleLoginKey(dto.ConsumeConsoleLoginKeyRequest{LoginKey: key.LoginKey}); !errors.Is(err, service.ErrUnauthorized) {
		t.Fatalf("expected single-use key to be rejected, got %v", err)
	}
}

func TestIssueAuthResponseUsesConfiguredAccessTTL(t *testing.T) {
	state := newNetworkTestState(t)
	state.cfg.Auth.AccessTokenTTLSeconds = 120
	state.cfg.Auth.RefreshTokenTTLSeconds = 240

	resp, err := state.issueAuthResponse(context.Background(), "user-1", "")
	if err != nil {
		t.Fatalf("issue auth response: %v", err)
	}
	if resp.ExpiresIn != 120 {
		t.Fatalf("expected configured access ttl, got %d", resp.ExpiresIn)
	}
	if _, err := state.tokens.AuthenticateRefreshToken(context.Background(), resp.RefreshToken); err != nil {
		t.Fatalf("expected refresh token to be stored: %v", err)
	}
}

func TestAuthenticate_AllowsDeviceBoundSessionWithoutControlSession(t *testing.T) {
	state := newNetworkTestState(t)

	tokenStore := state.tokens.(*memoryTokenStore)
	tokenStore.accessTokens["access-1"] = repo.AccessTokenSession{
		UserID:   "user-1",
		DeviceID: "dev-1",
		IssuedAt: time.Now().Add(-3 * time.Minute).Unix(),
	}

	userID, err := dbTokenVerifier{state: state}.Authenticate("access-1")
	if err != nil {
		t.Fatalf("authenticate device-bound session without control session: %v", err)
	}
	if userID != "user-1" {
		t.Fatalf("expected user-1, got %s", userID)
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
