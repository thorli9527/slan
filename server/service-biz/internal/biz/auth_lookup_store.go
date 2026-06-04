package biz

import (
	"context"
	"strings"
	"time"
)

func (s *Store) userByID(userID string) (User, error) {
	userID = strings.TrimSpace(userID)
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[userID]
	if !ok || user.Status != "active" {
		return User{}, errNotFound
	}
	return user, nil
}

func (s *Store) DeviceAuthByToken(token string) (DeviceSession, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return DeviceSession{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		return DeviceSession{}, err
	}
	sessionID, ok := s.deviceSessionByToken[token]
	if !ok {
		return DeviceSession{}, errUnauthorized
	}
	session, ok := s.deviceSessions[sessionID]
	if !ok || session.State != "active" || session.DeviceTokenExpiresAt < time.Now().Unix() {
		return DeviceSession{}, errUnauthorized
	}
	return session, nil
}
