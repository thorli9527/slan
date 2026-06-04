package biz

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"
)

func (s *Store) GetDevice(deviceID string) (Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		return Device{}, err
	}
	device, ok := s.devices[strings.TrimSpace(deviceID)]
	if !ok {
		return Device{}, errNotFound
	}
	return s.deviceWithOwnerEmailLocked(device), nil
}

func (s *Store) DeviceQuota(userID string) (DeviceQuota, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return DeviceQuota{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		return DeviceQuota{}, err
	}
	if _, ok := s.users[userID]; !ok {
		return DeviceQuota{}, errNotFound
	}
	return s.deviceQuotaLocked(userID), nil
}

func (s *Store) RemoveVisibleDevice(deviceID, actorUserID string) error {
	deviceID = strings.TrimSpace(deviceID)
	actorUserID = strings.TrimSpace(actorUserID)
	if deviceID == "" || actorUserID == "" {
		return errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	device, ok := s.devices[deviceID]
	if !ok {
		return errNotFound
	}
	if device.OwnerID != actorUserID {
		key := actorUserID + "|" + deviceID
		if grant, ok := s.deviceAccessGrants[key]; ok && grant.Status == "active" {
			grant.Status = "revoked"
			s.deviceAccessGrants[key] = grant
			if err := s.persistPostgresDeviceAccessGrantRevokeTxLocked(ctx, postgresTx, grant); err != nil {
				return err
			}
			postgresTx = nil
			return nil
		}
		return errNotFound
	}
	if err := s.removeDeviceLocked(deviceID); err != nil {
		return err
	}
	if err := s.persistPostgresDeviceDeleteTxLocked(ctx, postgresTx, deviceID); err != nil {
		return err
	}
	postgresTx = nil
	return nil
}

func (s *Store) removeDeviceLocked(deviceID string) error {
	if strings.TrimSpace(deviceID) == "" {
		return errBadRequest
	}
	if _, ok := s.devices[deviceID]; !ok {
		return errNotFound
	}
	delete(s.devices, deviceID)
	delete(s.runtimeStatuses, deviceID)
	for sessionID, session := range s.deviceSessions {
		if session.DeviceID == deviceID {
			delete(s.deviceSessions, sessionID)
			delete(s.deviceSessionByToken, session.DeviceToken)
		}
	}
	for key, owner := range s.deviceOwners {
		if owner.DeviceID == deviceID {
			delete(s.deviceOwners, key)
		}
	}
	for key, networkDevice := range s.networkDevices {
		if networkDevice.DeviceID == deviceID {
			delete(s.networkDevices, key)
		}
	}
	for key, grant := range s.deviceAccessGrants {
		if grant.DeviceID == deviceID {
			delete(s.deviceAccessGrants, key)
		}
	}
	for ipID, ip := range s.globalIPs {
		if ip.DeviceID == deviceID {
			ip.DeviceID = ""
			ip.Status = "available"
			ip.ReleasedAt = time.Now().Unix()
			s.globalIPs[ipID] = ip
		}
	}
	return nil
}

func (s *Store) ListVisibleDevices(userID string) []Device {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return s.ListDevices("")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres visible devices failed: %v", err)
	}
	deviceIDs := make(map[string]bool)
	for _, device := range s.devices {
		if device.OwnerID == userID {
			deviceIDs[device.DeviceID] = true
		}
	}
	for _, grant := range s.deviceAccessGrants {
		if grant.UserID == userID && grant.Status == "active" {
			deviceIDs[grant.DeviceID] = true
		}
	}
	out := make([]Device, 0, len(deviceIDs))
	for deviceID := range deviceIDs {
		if device, ok := s.devices[deviceID]; ok {
			out = append(out, s.deviceWithOwnerEmailLocked(device))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeviceID < out[j].DeviceID })
	return out
}

func (s *Store) UpdateDeviceAlias(deviceID, actorUserID, alias string) (Device, error) {
	deviceID = strings.TrimSpace(deviceID)
	actorUserID = strings.TrimSpace(actorUserID)
	alias = strings.TrimSpace(alias)
	if deviceID == "" || actorUserID == "" || alias == "" {
		return Device{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return Device{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	device, ok := s.devices[deviceID]
	if !ok {
		return Device{}, errNotFound
	}
	if !s.canUserSeeDeviceLocked(actorUserID, deviceID, device.OwnerID) {
		return Device{}, errNotFound
	}
	device.Alias = alias
	device.UpdatedAt = time.Now().Unix()
	s.devices[deviceID] = device
	if err := s.persistPostgresDeviceAliasTxLocked(ctx, postgresTx, device); err != nil {
		return Device{}, err
	}
	postgresTx = nil
	return s.deviceWithOwnerEmailLocked(device), nil
}

func (s *Store) ListUserAliases(ownerUserID string) []UserAlias {
	ownerUserID = strings.TrimSpace(ownerUserID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres user aliases failed: %v", err)
	}
	out := make([]UserAlias, 0)
	for _, alias := range s.userAliases {
		if ownerUserID == "" || alias.OwnerUserID == ownerUserID {
			out = append(out, alias)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Email < out[j].Email })
	return out
}

func (s *Store) UpsertUserAlias(ownerUserID, email, alias string) (UserAlias, error) {
	ownerUserID = strings.TrimSpace(ownerUserID)
	email = strings.ToLower(strings.TrimSpace(email))
	alias = strings.TrimSpace(alias)
	if ownerUserID == "" || email == "" || alias == "" {
		return UserAlias{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return UserAlias{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if _, ok := s.users[ownerUserID]; !ok {
		return UserAlias{}, errNotFound
	}
	item := UserAlias{OwnerUserID: ownerUserID, Email: email, Alias: alias, UpdatedAt: time.Now().Unix()}
	s.userAliases[ownerUserID+"|"+email] = item
	if err := s.persistPostgresUserAliasTxLocked(ctx, postgresTx, item); err != nil {
		return UserAlias{}, err
	}
	postgresTx = nil
	return item, nil
}

func (s *Store) ListDeviceOwnerLogs(deviceID string) []DeviceOwnerChangeLog {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres owner logs failed: %v", err)
	}
	out := make([]DeviceOwnerChangeLog, 0)
	for _, log := range s.ownerLogs {
		if deviceID == "" || log.DeviceID == deviceID {
			out = append(out, log)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ChangedAt < out[j].ChangedAt })
	return out
}

func (s *Store) ListGlobalIPs() []GlobalIPAddress {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres global ips failed: %v", err)
	}
	return sortedValues(s.globalIPs, func(a, b GlobalIPAddress) bool { return a.Offset < b.Offset })
}
