package biz

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

func (s *Store) CreateDeviceBootstrapKey(createdByUserID, networkID, deviceAlias string, ttlSeconds int64) (DeviceBootstrapKey, error) {
	createdByUserID = strings.TrimSpace(createdByUserID)
	networkID = strings.TrimSpace(networkID)
	if createdByUserID == "" || networkID == "" {
		return DeviceBootstrapKey{}, errBadRequest
	}
	if ttlSeconds <= 0 {
		ttlSeconds = int64((30 * time.Minute).Seconds())
	}
	if ttlSeconds > int64((24 * time.Hour).Seconds()) {
		ttlSeconds = int64((24 * time.Hour).Seconds())
	}
	keyValue, err := secureTokenHex(32)
	if err != nil {
		return DeviceBootstrapKey{}, err
	}
	keyValue = "sk_" + keyValue
	now := time.Now().Unix()
	s.mu.Lock()
	if _, ok := s.users[createdByUserID]; !ok {
		s.mu.Unlock()
		return DeviceBootstrapKey{}, errNotFound
	}
	network, ok := s.networks[networkID]
	if !ok || network.OwnerUserID != createdByUserID {
		s.mu.Unlock()
		return DeviceBootstrapKey{}, errNotFound
	}
	keyID := fmt.Sprintf("device-bootstrap-%06d", s.nextBootstrapKeySeq)
	s.nextBootstrapKeySeq++
	s.mu.Unlock()
	key := DeviceBootstrapKey{
		KeyID:           keyID,
		Key:             keyValue,
		KeyHash:         bootstrapKeyHash(keyValue),
		CreatedByUserID: createdByUserID,
		NetworkID:       networkID,
		DeviceAlias:     strings.TrimSpace(deviceAlias),
		CreatedAt:       now,
		ExpiresAt:       now + ttlSeconds,
		Status:          "unused",
	}
	ttl := time.Duration(ttlSeconds) * time.Second
	if err := s.deviceBootstrapKeyStore.SaveBootstrapKey(key, ttl); err != nil {
		return DeviceBootstrapKey{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deviceBootstrapKeys[key.KeyID] = key
	return key, nil
}

func (s *Store) ListDeviceBootstrapKeys(createdByUserID string) []DeviceBootstrapKey {
	createdByUserID = strings.TrimSpace(createdByUserID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres bootstrap keys failed: %v", err)
	}
	out := make([]DeviceBootstrapKey, 0)
	now := time.Now().Unix()
	for _, key := range s.deviceBootstrapKeys {
		if createdByUserID != "" && key.CreatedByUserID != createdByUserID {
			continue
		}
		key.Key = ""
		if key.Status == "unused" && key.ExpiresAt < now {
			key.Status = "expired"
		}
		out = append(out, key)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func (s *Store) RevokeDeviceBootstrapKey(keyID, actorUserID string) (DeviceBootstrapKey, error) {
	keyID = strings.TrimSpace(keyID)
	actorUserID = strings.TrimSpace(actorUserID)
	if keyID == "" || actorUserID == "" {
		return DeviceBootstrapKey{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return DeviceBootstrapKey{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	key, ok := s.deviceBootstrapKeys[keyID]
	if !ok || key.CreatedByUserID != actorUserID {
		return DeviceBootstrapKey{}, errNotFound
	}
	if key.Status == "used" {
		return DeviceBootstrapKey{}, errConflict
	}
	key.Status = "revoked"
	key.RevokedAt = time.Now().Unix()
	key.Key = ""
	s.deviceBootstrapKeys[keyID] = key
	if err := s.persistPostgresBootstrapKeyRevokeTxLocked(ctx, postgresTx, key); err != nil {
		return DeviceBootstrapKey{}, err
	}
	postgresTx = nil
	return key, nil
}
