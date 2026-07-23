package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

type userSessionRotationUsers struct {
	repository.UserRepository
	user model.User
}

func (s userSessionRotationUsers) GetUser(_ context.Context, userID string) (model.User, bool, error) {
	return s.user, userID == s.user.UserID, nil
}

func (s userSessionRotationUsers) GetByEmail(_ context.Context, email string) (model.User, bool, error) {
	return s.user, email == s.user.Email, nil
}

type userSessionRotationStore struct {
	repository.UserSessionRepository
	sessions map[string]model.UserSession
}

func (s *userSessionRotationStore) GetUserSessionByAccessToken(_ context.Context, accessToken string) (model.UserSession, bool, error) {
	for _, session := range s.sessions {
		if session.AccessToken == accessToken {
			return session, true, nil
		}
	}
	return model.UserSession{}, false, nil
}

func (s *userSessionRotationStore) GetUserSessionByRefreshToken(_ context.Context, refreshToken string) (model.UserSession, bool, error) {
	digest := sha256.Sum256([]byte(refreshToken))
	refreshTokenHash := hex.EncodeToString(digest[:])
	for _, session := range s.sessions {
		if session.RefreshToken == refreshToken || session.PreviousRefreshTokenHash == refreshTokenHash {
			return session, true, nil
		}
	}
	return model.UserSession{}, false, nil
}

func TestRenewUserSessionRetriesPreviousTokenIdempotently(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	store := &userSessionRotationStore{sessions: map[string]model.UserSession{
		"session-a": {
			SessionID: "session-a", UserID: "user-1", AccessToken: "access-a", RefreshToken: "refresh-a",
			Status: tokenStatusActive, SessionMode: tokenModeLong, ClientType: UserSessionClientDesktop,
			DeviceID: "device-a", ExpiresAt: now.Add(time.Hour).Unix(), RefreshExpiry: now.Add(24 * time.Hour).Unix(),
		},
	}}
	service := AuthUserSessionService{authUserDependencies: authUserDependencies{
		Users:    userSessionRotationUsers{user: model.User{UserID: "user-1", Email: "user@example.test"}},
		Sessions: store, NewSessID: func(string) string { return "session-renewed" }, Now: func() time.Time { return now },
	}}

	first, err := service.RenewUserSession(context.Background(), "access-a", RenewUserSessionInput{RefreshToken: "refresh-a"})
	if err != nil {
		t.Fatalf("first renewal: %v", err)
	}
	retry, err := service.RenewUserSession(context.Background(), "access-a", RenewUserSessionInput{RefreshToken: "refresh-a"})
	if err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	if retry.Session.AccessToken != first.Session.AccessToken || retry.Session.RefreshToken != first.Session.RefreshToken {
		t.Fatalf("retry rotated tokens again: first=%#v retry=%#v", first.Session, retry.Session)
	}

	service.Now = func() time.Time { return now.Add(userRefreshRotationGrace + time.Second) }
	if _, err := service.RenewUserSession(context.Background(), "access-a", RenewUserSessionInput{RefreshToken: "refresh-a"}); err != ErrUnauthorized {
		t.Fatalf("expired rotation retry error = %v, want unauthorized", err)
	}
}

func (s *userSessionRotationStore) ReplaceUserSession(_ context.Context, oldAccessToken string, next model.UserSession) error {
	for id, session := range s.sessions {
		if session.AccessToken == oldAccessToken {
			delete(s.sessions, id)
		}
	}
	s.sessions[next.SessionID] = next
	return nil
}

func (s *userSessionRotationStore) ReplaceUserSessionForClient(_ context.Context, next model.UserSession) error {
	for id, session := range s.sessions {
		if session.UserID == next.UserID && session.ClientType == next.ClientType && session.DeviceID == next.DeviceID {
			delete(s.sessions, id)
		}
	}
	s.sessions[next.SessionID] = next
	return nil
}

func (s *userSessionRotationStore) SaveUserSession(_ context.Context, next model.UserSession) error {
	s.sessions[next.SessionID] = next
	return nil
}

func (s *userSessionRotationStore) DeleteUserSessionByAccessToken(_ context.Context, accessToken string) error {
	for id, session := range s.sessions {
		if session.AccessToken == accessToken {
			delete(s.sessions, id)
		}
	}
	return nil
}

func TestRenewUserSessionPreservesOtherClientSessions(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	store := &userSessionRotationStore{sessions: map[string]model.UserSession{
		"session-a": {
			SessionID: "session-a", UserID: "user-1", AccessToken: "access-a", RefreshToken: "refresh-a",
			Status: tokenStatusActive, SessionMode: tokenModeLong, ExpiresAt: now.Add(time.Hour).Unix(), RefreshExpiry: now.Add(24 * time.Hour).Unix(),
		},
		"session-b": {
			SessionID: "session-b", UserID: "user-1", AccessToken: "access-b", RefreshToken: "refresh-b",
			Status: tokenStatusActive, SessionMode: tokenModeLong, ExpiresAt: now.Add(time.Hour).Unix(), RefreshExpiry: now.Add(24 * time.Hour).Unix(),
		},
	}}
	service := AuthUserSessionService{
		authUserDependencies: authUserDependencies{
			Users:    userSessionRotationUsers{user: model.User{UserID: "user-1", Email: "user@example.test"}},
			Sessions: store,
			NewSessID: func(string) string {
				return "session-a-renewed"
			},
			Now: func() time.Time { return now },
		},
	}

	view, err := service.RenewUserSession(context.Background(), "access-a", RenewUserSessionInput{RefreshToken: "refresh-a"})
	if err != nil {
		t.Fatalf("renew session A: %v", err)
	}
	if view.Session.SessionID != "session-a-renewed" {
		t.Fatalf("unexpected renewed session: %#v", view)
	}
	if _, ok, _ := store.GetUserSessionByAccessToken(context.Background(), "access-a"); ok {
		t.Fatal("old session A access token still exists")
	}
	if _, ok, _ := store.GetUserSessionByAccessToken(context.Background(), "access-b"); !ok {
		t.Fatal("renewing session A removed independent client session B")
	}
	if _, ok, _ := store.GetUserSessionByRefreshToken(context.Background(), "refresh-b"); !ok {
		t.Fatal("renewing session A removed independent client B refresh token")
	}
}

func TestLoginUserReplacesOnlyTheSameClientDevice(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	passwordHash := hashPassword("password-1")
	store := &userSessionRotationStore{sessions: make(map[string]model.UserSession)}
	nextID := 0
	service := AuthUserSessionService{
		authUserDependencies: authUserDependencies{
			Users: userSessionRotationUsers{user: model.User{
				UserID: "user-1", Email: "user@example.test", PasswordHash: passwordHash,
			}},
			Sessions: store,
			NewSessID: func(string) string {
				nextID++
				return "session-" + string(rune('0'+nextID))
			},
			Now: func() time.Time { return now },
		},
	}

	firstDesktop, err := service.LoginUser(context.Background(), LoginUserInput{
		Email: "user@example.test", Password: "password-1", ClientType: UserSessionClientDesktop, DeviceID: "device-a",
	})
	if err != nil {
		t.Fatalf("first desktop login: %v", err)
	}
	web, err := service.LoginUser(context.Background(), LoginUserInput{
		Email: "user@example.test", Password: "password-1", ClientType: UserSessionClientWeb, SessionMode: tokenModeLong,
	})
	if err != nil {
		t.Fatalf("web login: %v", err)
	}
	latestDesktop, err := service.LoginUser(context.Background(), LoginUserInput{
		Email: "user@example.test", Password: "password-1", ClientType: UserSessionClientDesktop, DeviceID: "device-a",
	})
	if err != nil {
		t.Fatalf("second desktop login: %v", err)
	}

	if _, ok, _ := store.GetUserSessionByAccessToken(context.Background(), firstDesktop.Session.AccessToken); ok {
		t.Fatal("old desktop session remains valid")
	}
	if _, ok, _ := store.GetUserSessionByAccessToken(context.Background(), latestDesktop.Session.AccessToken); !ok {
		t.Fatal("latest desktop session is missing")
	}
	if _, ok, _ := store.GetUserSessionByAccessToken(context.Background(), web.Session.AccessToken); !ok {
		t.Fatal("desktop login replaced the independent web session")
	}
	if web.Session.SessionMode != tokenModeShort {
		t.Fatalf("web session mode = %q, want short", web.Session.SessionMode)
	}
	otherDesktop, err := service.LoginUser(context.Background(), LoginUserInput{
		Email: "user@example.test", Password: "password-1", ClientType: UserSessionClientDesktop, DeviceID: "device-b",
	})
	if err != nil {
		t.Fatalf("other desktop login: %v", err)
	}
	if _, ok, _ := store.GetUserSessionByAccessToken(context.Background(), latestDesktop.Session.AccessToken); !ok {
		t.Fatal("other desktop device replaced device A session")
	}
	if _, ok, _ := store.GetUserSessionByAccessToken(context.Background(), otherDesktop.Session.AccessToken); !ok {
		t.Fatal("device B desktop session is missing")
	}
	if len(store.sessions) != 3 {
		t.Fatalf("session count = %d, want one web and two desktop device sessions", len(store.sessions))
	}
}

func TestLogoutUserPreservesIndependentDeviceSession(t *testing.T) {
	store := &userSessionRotationStore{sessions: map[string]model.UserSession{
		"desktop": {SessionID: "desktop", AccessToken: "user-access", UserID: "user-1"},
	}}
	service := AuthUserSessionService{authUserDependencies: authUserDependencies{Sessions: store}}

	if err := service.LogoutUser(context.Background(), "user-access", LogoutUserInput{DeviceToken: "device-access"}); err != nil {
		t.Fatalf("logout user: %v", err)
	}
	if _, ok, _ := store.GetUserSessionByAccessToken(context.Background(), "user-access"); ok {
		t.Fatal("user session remains after logout")
	}
}
