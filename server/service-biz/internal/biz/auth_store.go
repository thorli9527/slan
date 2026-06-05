package biz

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *Store) RegisterUser(email, password, name string) (AuthResponse, Network, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || strings.TrimSpace(password) == "" {
		return AuthResponse{}, Network{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return AuthResponse{}, Network{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if _, ok := s.userByEmail[email]; ok {
		return AuthResponse{}, Network{}, errConflict
	}
	now := time.Now().Unix()
	user := User{
		UserID:       fmt.Sprintf("user-%06d", s.nextUserID),
		Email:        email,
		Name:         strings.TrimSpace(name),
		PasswordHash: hashPassword(password),
		Status:       "active",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.nextUserID++
	s.users[user.UserID] = user
	s.userByEmail[email] = user.UserID
	network, group := s.ensureDefaultNetworkResourcesForUserLocked(user.UserID, now)
	session := s.createSessionLocked(user.UserID, now)
	if err := s.persistPostgresUserRegisterTxLocked(ctx, postgresTx, user, network, group, session); err != nil {
		return AuthResponse{}, Network{}, err
	}
	postgresTx = nil
	return AuthResponse{User: user, Session: session}, network, nil
}

func (s *Store) LoginUser(email, password string) (AuthResponse, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return AuthResponse{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	userID, ok := s.userByEmail[email]
	if !ok {
		return AuthResponse{}, errNotFound
	}
	user := s.users[userID]
	if !verifyPasswordHash(user.PasswordHash, password) || user.Status != "active" {
		return AuthResponse{}, errBadRequest
	}
	session := s.createSessionLocked(user.UserID, time.Now().Unix())
	if err := s.persistPostgresUserSessionTxLocked(ctx, postgresTx, session); err != nil {
		return AuthResponse{}, err
	}
	postgresTx = nil
	return AuthResponse{User: user, Session: session}, nil
}
