package biz

import (
	"sync"
	"time"
)

type memoryDeviceInviteStore struct {
	mu            sync.Mutex
	invites       map[string]DeviceInvite
	bootstrapKeys map[string]DeviceBootstrapKey
}

func newMemoryDeviceInviteStore() *memoryDeviceInviteStore {
	return &memoryDeviceInviteStore{
		invites:       make(map[string]DeviceInvite),
		bootstrapKeys: make(map[string]DeviceBootstrapKey),
	}
}

func (s *memoryDeviceInviteStore) Save(invite DeviceInvite, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	invite.ExpiresAt = time.Now().Add(ttl).Unix()
	s.invites[invite.InviteCode] = invite
	return nil
}

func (s *memoryDeviceInviteStore) Consume(inviteCode string) (DeviceInvite, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	invite, ok := s.invites[inviteCode]
	if !ok || invite.Status != "pending" || invite.ExpiresAt < time.Now().Unix() {
		return DeviceInvite{}, errNotFound
	}
	delete(s.invites, inviteCode)
	return invite, nil
}

func (s *memoryDeviceInviteStore) SaveBootstrapKey(key DeviceBootstrapKey, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if key.ExpiresAt <= 0 {
		key.ExpiresAt = time.Now().Add(ttl).Unix()
	}
	if _, ok := s.bootstrapKeys[key.KeyHash]; ok {
		return errConflict
	}
	s.bootstrapKeys[key.KeyHash] = key
	return nil
}

func (s *memoryDeviceInviteStore) ConsumeBootstrapKey(keyHash string) (DeviceBootstrapKey, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key, ok := s.bootstrapKeys[keyHash]
	if !ok || key.Status != "unused" || key.ExpiresAt < time.Now().Unix() || key.RevokedAt > 0 {
		return DeviceBootstrapKey{}, errNotFound
	}
	delete(s.bootstrapKeys, keyHash)
	return key, nil
}
