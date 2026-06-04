package biz

import (
	"context"
	"strings"
	"time"
)

func (s *Store) AuthByToken(token string) (AuthResponse, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return AuthResponse{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		return AuthResponse{}, err
	}
	for _, session := range s.sessions {
		if session.Token != token {
			continue
		}
		if session.ExpiresAt < time.Now().Unix() {
			return AuthResponse{}, errNotFound
		}
		user, ok := s.users[session.UserID]
		if !ok || user.Status != "active" {
			return AuthResponse{}, errNotFound
		}
		return AuthResponse{User: user, Session: session}, nil
	}
	return AuthResponse{}, errNotFound
}

func (s *Store) RenewUserSession(token string) (AuthResponse, error) {
	token = strings.TrimSpace(token)
	if token == "" {
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
	session, ok := s.sessions[token]
	if !ok || session.ExpiresAt < time.Now().Unix() {
		return AuthResponse{}, errNotFound
	}
	user, ok := s.users[session.UserID]
	if !ok || user.Status != "active" {
		return AuthResponse{}, errNotFound
	}
	session.ExpiresAt = time.Now().Add(userSessionTTL).Unix()
	s.sessions[token] = session
	if err := s.persistPostgresUserSessionRenewTxLocked(ctx, postgresTx, session); err != nil {
		return AuthResponse{}, err
	}
	postgresTx = nil
	return AuthResponse{User: user, Session: session}, nil
}
