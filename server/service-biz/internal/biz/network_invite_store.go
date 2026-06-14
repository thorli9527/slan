package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log"
	"sort"
	"strings"
	"time"
)

func (s *Store) CreateDeviceInvite(inviterUserID string, ttlSeconds int64) (DeviceInvite, error) {
	inviterUserID = strings.TrimSpace(inviterUserID)
	if inviterUserID == "" {
		return DeviceInvite{}, errBadRequest
	}
	s.mu.Lock()
	if _, ok := s.users[inviterUserID]; !ok {
		s.mu.Unlock()
		return DeviceInvite{}, errNotFound
	}
	if quota := s.deviceQuotaLocked(inviterUserID); quota.TotalDeviceLimit > 0 && quota.RemainingDevices <= 0 {
		s.mu.Unlock()
		return DeviceInvite{}, errConflict
	}
	now := time.Now().Unix()
	code, err := secureInviteCode()
	if err != nil {
		s.mu.Unlock()
		return DeviceInvite{}, err
	}
	ttl := deviceInviteTTL
	if ttlSeconds > 0 && ttlSeconds < int64(deviceInviteTTL.Seconds()) {
		ttl = time.Duration(ttlSeconds) * time.Second
	}
	invite := DeviceInvite{
		InviteID:      newCompactUUID(),
		InviterUserID: inviterUserID,
		InviteCode:    code,
		Status:        "pending",
		CreatedAt:     now,
		ExpiresAt:     now + int64(ttl.Seconds()),
	}
	s.nextDeviceInviteSeq++
	s.mu.Unlock()
	if err := s.deviceInviteStore.Save(invite, ttl); err != nil {
		return DeviceInvite{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deviceInvites[invite.InviteCode] = invite
	return invite, nil
}

func (s *Store) ListDeviceInvites(inviterUserID string) []DeviceInvite {
	inviterUserID = strings.TrimSpace(inviterUserID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres device invites failed: %v", err)
	}
	out := make([]DeviceInvite, 0)
	for _, invite := range s.deviceInvites {
		if inviterUserID == "" || invite.InviterUserID == inviterUserID {
			out = append(out, invite)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	return out
}

func secureInviteCode() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(raw[:])), nil
}

func (s *Store) AcceptDeviceInvite(inviteCode, deviceID, actorUserID string) (DeviceAccessGrant, DeviceInvite, error) {
	inviteCode = strings.ToUpper(strings.TrimSpace(inviteCode))
	deviceID = strings.TrimSpace(deviceID)
	actorUserID = strings.TrimSpace(actorUserID)
	if inviteCode == "" || deviceID == "" || actorUserID == "" {
		return DeviceAccessGrant{}, DeviceInvite{}, errBadRequest
	}
	invite, err := s.deviceInviteStore.Consume(inviteCode)
	if err != nil {
		return DeviceAccessGrant{}, DeviceInvite{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return DeviceAccessGrant{}, DeviceInvite{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	device, ok := s.devices[deviceID]
	if !ok {
		return DeviceAccessGrant{}, DeviceInvite{}, errNotFound
	}
	if device.OwnerID != actorUserID {
		return DeviceAccessGrant{}, DeviceInvite{}, errBadRequest
	}
	if invite.InviterUserID == actorUserID {
		return DeviceAccessGrant{}, DeviceInvite{}, errConflict
	}
	if quota := s.deviceQuotaLocked(invite.InviterUserID); quota.TotalDeviceLimit > 0 && quota.RemainingDevices <= 0 {
		return DeviceAccessGrant{}, DeviceInvite{}, errConflict
	}
	key := invite.InviterUserID + "|" + deviceID
	if existing, ok := s.deviceAccessGrants[key]; ok && existing.Status == "active" {
		return DeviceAccessGrant{}, DeviceInvite{}, errConflict
	}
	now := time.Now().Unix()
	grant := DeviceAccessGrant{
		GrantID:    newCompactUUID(),
		DeviceID:   deviceID,
		UserID:     invite.InviterUserID,
		GrantedBy:  actorUserID,
		InviteCode: inviteCode,
		Status:     "active",
		CreatedAt:  now,
	}
	s.deviceAccessGrants[key] = grant
	invite.Status = "accepted"
	invite.AcceptedDeviceID = deviceID
	invite.AcceptedUserID = actorUserID
	invite.AcceptedAt = now
	s.deviceInvites[inviteCode] = invite
	if err := s.persistPostgresDeviceInviteAcceptTxLocked(ctx, postgresTx, invite, grant); err != nil {
		return DeviceAccessGrant{}, DeviceInvite{}, err
	}
	postgresTx = nil
	return grant, invite, nil
}
