package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
)

func TestRotateDeviceSessionConsumesCurrentRefreshTokenOnce(t *testing.T) {
	store, err := OpenGormStore(GormConfig{})
	if err != nil {
		if strings.Contains(err.Error(), "failed to connect") || strings.Contains(err.Error(), "dial error") {
			t.Skipf("postgres is not available for repository integration test: %v", err)
		}
		t.Fatalf("open gorm store: %v", err)
	}
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	now := time.Unix(1_700_000_000, 0).Unix()
	current := model.DeviceSession{
		SessionID: "session-current", DeviceID: "device-1", AccessToken: "access-current", RefreshToken: "refresh-current",
		Status: "active", SessionMode: "long", ExpiresAt: now + 3600, RefreshExpiry: now + 86400, CreatedAt: now, UpdatedAt: now,
	}
	if err := store.SaveDeviceSession(context.Background(), current); err != nil {
		t.Fatalf("save current session: %v", err)
	}

	winner := current
	winner.SessionID = "session-winner"
	winner.AccessToken = "access-winner"
	winner.RefreshToken = "refresh-winner"
	winner.PreviousRefreshTokenHash = "previous-hash"
	rotated, err := store.RotateDeviceSession(context.Background(), current.RefreshToken, winner)
	if err != nil || !rotated {
		t.Fatalf("first rotation: rotated=%v err=%v", rotated, err)
	}

	loser := winner
	loser.SessionID = "session-loser"
	loser.AccessToken = "access-loser"
	loser.RefreshToken = "refresh-loser"
	rotated, err = store.RotateDeviceSession(context.Background(), current.RefreshToken, loser)
	if err != nil {
		t.Fatalf("second rotation: %v", err)
	}
	if rotated {
		t.Fatal("consumed refresh token rotated a second time")
	}
	stored, found, err := store.GetDeviceSessionByAccessToken(context.Background(), winner.AccessToken)
	if err != nil || !found {
		t.Fatalf("get winning session: found=%v err=%v", found, err)
	}
	if stored.RefreshToken != winner.RefreshToken {
		t.Fatalf("winning session was overwritten: %#v", stored)
	}
	digest := sha256.Sum256([]byte(current.RefreshToken))
	previousHash := hex.EncodeToString(digest[:])
	stored.PreviousRefreshTokenHash = previousHash
	stored.RefreshRotationGraceExpiry = now - 1
	if err := store.SaveDeviceSession(context.Background(), stored); err != nil {
		t.Fatalf("save expired rotation grace: %v", err)
	}
	deleted, err := store.DeleteDeviceSessionForRefreshReuse(context.Background(), stored.SessionID, previousHash, now)
	if err != nil || !deleted {
		t.Fatalf("delete refresh reuse: deleted=%v err=%v", deleted, err)
	}
	if _, found, err := store.GetDeviceSessionByAccessToken(context.Background(), stored.AccessToken); err != nil || found {
		t.Fatalf("reused refresh session still exists: found=%v err=%v", found, err)
	}
}
