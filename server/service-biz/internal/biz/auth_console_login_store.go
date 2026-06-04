package biz

import (
	"context"
	"strings"
	"time"
)

func (s *Store) CreateConsoleLoginKey(accessToken, deviceID string, ttl time.Duration) (ConsoleLoginKey, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return ConsoleLoginKey{}, errBadRequest
	}
	auth, err := s.AuthByToken(accessToken)
	if err != nil {
		return ConsoleLoginKey{}, err
	}
	if ttl <= 0 {
		ttl = 2 * time.Minute
	}
	token, err := secureTokenHex(24)
	if err != nil {
		return ConsoleLoginKey{}, err
	}
	now := time.Now().Unix()
	key := ConsoleLoginKey{
		LoginKey:  "clk-" + token,
		UserID:    auth.User.UserID,
		DeviceID:  deviceID,
		CreatedAt: now,
		ExpiresAt: now + int64(ttl.Seconds()),
		Status:    "unused",
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return ConsoleLoginKey{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	device, ok := s.devices[deviceID]
	if !ok || device.OwnerID != auth.User.UserID || device.Status != "active" {
		return ConsoleLoginKey{}, errNotFound
	}
	s.consoleLoginKeys[key.LoginKey] = key
	if err := s.persistPostgresConsoleLoginKeyTxLocked(ctx, postgresTx, key); err != nil {
		return ConsoleLoginKey{}, err
	}
	postgresTx = nil
	return key, nil
}

func (s *Store) ConsumeConsoleLoginKey(loginKey string) (AuthResponse, error) {
	loginKey = strings.TrimSpace(loginKey)
	if loginKey == "" {
		return AuthResponse{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return AuthResponse{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	key, ok := s.consoleLoginKeys[loginKey]
	if !ok {
		return AuthResponse{}, errNotFound
	}
	now := time.Now().Unix()
	if key.Status != "unused" || key.ExpiresAt < now {
		return AuthResponse{}, errNotFound
	}
	user, ok := s.users[key.UserID]
	if !ok || user.Status != "active" {
		return AuthResponse{}, errNotFound
	}
	key.Status = "used"
	key.ConsumedAt = now
	s.consoleLoginKeys[loginKey] = key
	session := s.createSessionLocked(user.UserID, now)
	if err := s.persistPostgresConsoleLoginConsumeTxLocked(ctx, postgresTx, key, session); err != nil {
		return AuthResponse{}, err
	}
	postgresTx = nil
	return AuthResponse{User: user, Session: session}, nil
}
