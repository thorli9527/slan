package biz

import (
	"context"
	"strings"
	"time"
)

func (s *Store) BootstrapDeviceSession(sessionKey, deviceID, name, platform, osName, osVersion, alias, publicKey string) (Device, DeviceSession, []NetworkConfig, error) {
	sessionKey = strings.TrimSpace(sessionKey)
	deviceID = strings.TrimSpace(deviceID)
	if sessionKey == "" || deviceID == "" {
		return Device{}, DeviceSession{}, nil, errBadRequest
	}
	bootstrapKey, err := s.deviceBootstrapKeyStore.ConsumeBootstrapKey(bootstrapKeyHash(sessionKey))
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	now := time.Now().Unix()
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if stored, ok := s.deviceBootstrapKeys[bootstrapKey.KeyID]; ok {
		if stored.Status != "unused" || stored.RevokedAt > 0 || stored.ExpiresAt < now {
			return Device{}, DeviceSession{}, nil, errNotFound
		}
	}
	user, ok := s.users[bootstrapKey.CreatedByUserID]
	if !ok || user.Status != "active" {
		return Device{}, DeviceSession{}, nil, errNotFound
	}
	network, ok := s.networks[bootstrapKey.NetworkID]
	if !ok || network.OwnerUserID != user.UserID {
		return Device{}, DeviceSession{}, nil, errNotFound
	}
	if existing, ok := s.devices[deviceID]; ok && existing.OwnerID != user.UserID && !samePhysicalDeviceIdentity(existing, publicKey) {
		return Device{}, DeviceSession{}, nil, errConflict
	}
	device := s.upsertBootstrapDeviceLocked(user.UserID, deviceID, name, platform, osName, osVersion, defaultString(alias, bootstrapKey.DeviceAlias), publicKey, now)
	membership := s.addNetworkDeviceLocked(network.NetworkID, device.DeviceID, user.UserID, defaultString(alias, bootstrapKey.DeviceAlias), true, now)
	configs, err := s.networkConfigsForDeviceLocked(device.DeviceID)
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	session, err := s.createDeviceSessionLocked(device.DeviceID, user.UserID, configs, now)
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	bootstrapKey.Status = "used"
	bootstrapKey.UsedAt = now
	bootstrapKey.UsedByDeviceID = device.DeviceID
	bootstrapKey.Key = ""
	s.deviceBootstrapKeys[bootstrapKey.KeyID] = bootstrapKey
	if err := s.persistPostgresBootstrapDeviceSessionTxLocked(ctx, postgresTx, device, membership, session, bootstrapKey); err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	postgresTx = nil
	return s.deviceWithOwnerEmailLocked(device), session, configs, nil
}

func (s *Store) BindDeviceSession(accessToken, deviceID, name, platform, osName, osVersion, alias, publicKey string) (Device, DeviceSession, []NetworkConfig, error) {
	auth, err := s.AuthByToken(accessToken)
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return Device{}, DeviceSession{}, nil, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	device, ok := s.devices[deviceID]
	previousOwnerID := ""
	if ok && device.OwnerID != auth.User.UserID {
		if !samePhysicalDeviceIdentity(device, publicKey) {
			return Device{}, DeviceSession{}, nil, errConflict
		}
		previousOwnerID = device.OwnerID
		s.transferDeviceOwnerLocked(deviceID, device.OwnerID, auth.User.UserID, "device_session_bind", time.Now().Unix())
		s.removeDeviceFromUserNetworksLocked(deviceID, device.OwnerID)
		s.revokeDeviceSessionsLocked(deviceID, device.OwnerID)
		device.OwnerID = auth.User.UserID
		device.Status = "active"
		device.UpdatedAt = time.Now().Unix()
		s.devices[deviceID] = device
	}
	if !ok {
		device = s.upsertBootstrapDeviceLocked(auth.User.UserID, deviceID, name, platform, osName, osVersion, alias, publicKey, time.Now().Unix())
		defaultNetwork := s.ensureDefaultNetworkForUserLocked(auth.User.UserID, time.Now().Unix())
		s.addNetworkDeviceLocked(defaultNetwork.NetworkID, deviceID, auth.User.UserID, alias, true, time.Now().Unix())
	} else {
		device.Status = "active"
		device.UpdatedAt = time.Now().Unix()
		s.devices[deviceID] = device
		s.addDeviceOwnerLocked(deviceID, auth.User.UserID, time.Now().Unix())
	}
	configs, err := s.networkConfigsForDeviceLocked(deviceID)
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	s.revokeDeviceSessionsLocked(deviceID, auth.User.UserID)
	session, err := s.createDeviceSessionLocked(deviceID, auth.User.UserID, configs, time.Now().Unix())
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	membership := NetworkDevice{}
	for _, item := range s.networkDevices {
		if item.DeviceID == deviceID && item.OwnerUserID == auth.User.UserID && item.Enabled && item.Status == "active" {
			membership = item
			break
		}
	}
	if membership.NetworkDeviceID == "" {
		defaultNetwork := s.ensureDefaultNetworkForUserLocked(auth.User.UserID, time.Now().Unix())
		membership = s.addNetworkDeviceLocked(defaultNetwork.NetworkID, deviceID, auth.User.UserID, alias, true, time.Now().Unix())
	}
	if err := s.persistPostgresDeviceBindSessionTxLocked(ctx, postgresTx, device, membership, session, previousOwnerID); err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	postgresTx = nil
	return s.deviceWithOwnerEmailLocked(device), session, configs, nil
}

func samePhysicalDeviceIdentity(existing Device, publicKey string) bool {
	existingKey := strings.TrimSpace(existing.PublicKey)
	nextKey := strings.TrimSpace(publicKey)
	return existingKey != "" && nextKey != "" && existingKey == nextKey
}

func (s *Store) RenewDeviceSession(deviceToken string, networkEnabled bool, rxBytesTotal, txBytesTotal uint64) (Device, DeviceSession, []NetworkConfig, error) {
	deviceToken = strings.TrimSpace(deviceToken)
	if deviceToken == "" {
		return Device{}, DeviceSession{}, nil, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	sessionID, ok := s.deviceSessionByToken[deviceToken]
	if !ok {
		return Device{}, DeviceSession{}, nil, errUnauthorized
	}
	session, ok := s.deviceSessions[sessionID]
	if !ok || session.State != "active" || session.DeviceTokenExpiresAt < time.Now().Unix() {
		return Device{}, DeviceSession{}, nil, errUnauthorized
	}
	device, ok := s.devices[session.DeviceID]
	if !ok {
		return Device{}, DeviceSession{}, nil, errNotFound
	}
	now := time.Now().Unix()
	device.Status = "active"
	device.UpdatedAt = now
	s.devices[device.DeviceID] = device
	status := s.runtimeStatuses[device.DeviceID]
	status.DeviceID = device.DeviceID
	status.HeartbeatOnline = true
	status.NetworkEnabled = networkEnabled
	status.DeviceEnabled = true
	status.RxBytesTotal = rxBytesTotal
	status.TxBytesTotal = txBytesTotal
	status.LastSeenAt = now
	status.LastReportAt = now
	s.runtimeStatuses[device.DeviceID] = status
	configs, err := s.networkConfigsForDeviceLocked(device.DeviceID)
	if err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	session.LastRenewedAt = now
	session.DeviceTokenExpiresAt = now + int64(deviceSessionTTL.Seconds())
	session.ActiveNetworkIDs = networkIDsFromConfigs(configs)
	s.deviceSessions[session.SessionID] = session
	if err := s.persistPostgresDeviceSessionRenewTxLocked(ctx, postgresTx, device, status, session); err != nil {
		return Device{}, DeviceSession{}, nil, err
	}
	postgresTx = nil
	return s.deviceWithOwnerEmailLocked(device), session, configs, nil
}

func (s *Store) CompleteDeviceLoginForDevice(deviceID, accessToken, action string) (DeviceUserLoginPayload, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return DeviceUserLoginPayload{}, errBadRequest
	}
	auth, err := s.AuthByToken(accessToken)
	if err != nil {
		return DeviceUserLoginPayload{}, err
	}
	if err := s.bindExistingDeviceToUser(deviceID, auth.User.UserID); err != nil {
		return DeviceUserLoginPayload{}, err
	}
	refreshToken := auth.Session.Token
	return DeviceUserLoginPayload{
		AccessToken:  auth.Session.Token,
		UserToken:    auth.Session.Token,
		RefreshToken: &refreshToken,
		UserID:       auth.User.UserID,
		UserLabel:    defaultString(auth.User.Email, auth.User.UserID),
		DeviceID:     optionalStringPtr(deviceID),
		ExpiresIn:    uint64(maxInt64(auth.Session.ExpiresAt-time.Now().Unix(), 0)),
		Action:       defaultString(action, "login"),
	}, nil
}

func (s *Store) ChangeUserPassword(userID, oldPassword, newPassword string) error {
	userID = strings.TrimSpace(userID)
	if userID == "" || strings.TrimSpace(oldPassword) == "" || strings.TrimSpace(newPassword) == "" {
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
	user, ok := s.users[userID]
	if !ok {
		return errNotFound
	}
	if !verifyPasswordHash(user.PasswordHash, oldPassword) {
		return errBadRequest
	}
	user.PasswordHash = hashPassword(newPassword)
	user.UpdatedAt = time.Now().Unix()
	s.users[userID] = user
	if err := s.persistPostgresUserPasswordTxLocked(ctx, postgresTx, user); err != nil {
		return err
	}
	postgresTx = nil
	return nil
}
