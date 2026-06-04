package biz

import (
	"context"
	"database/sql"
	"strings"
	"time"
)

func (s *Store) PrepareDeviceLoginDevice(deviceID, name, platform, osName, osVersion, alias, publicKey string) (Device, error) {
	deviceID = strings.TrimSpace(deviceID)
	publicKey = strings.TrimSpace(publicKey)
	if deviceID == "" {
		return Device{}, errBadRequest
	}
	if !validPrepareDevicePublicKey(deviceID, publicKey) {
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
	if err := s.expirePreloginDevicesLocked(ctx, postgresTx, time.Now().Unix()); err != nil {
		return Device{}, err
	}
	if existing, ok := s.devices[deviceID]; ok {
		if strings.TrimSpace(existing.OwnerID) != "" {
			if strings.TrimSpace(existing.PublicKey) != "" && strings.TrimSpace(existing.PublicKey) != publicKey {
				return Device{}, errConflict
			}
			return s.deviceWithOwnerEmailLocked(existing), nil
		}
		existing.Name = defaultString(strings.TrimSpace(name), existing.Name)
		existing.Platform = defaultString(strings.TrimSpace(platform), existing.Platform)
		existing.OSName = defaultString(strings.TrimSpace(osName), existing.OSName)
		existing.OSVersion = defaultString(strings.TrimSpace(osVersion), existing.OSVersion)
		existing.Alias = defaultString(strings.TrimSpace(alias), existing.Alias)
		existing.PublicKey = defaultString(publicKey, existing.PublicKey)
		existing.Status = "active"
		existing.UpdatedAt = time.Now().Unix()
		s.devices[deviceID] = existing
		if err := s.persistPostgresDeviceRegisterTxLocked(ctx, postgresTx, existing, NetworkDevice{}, ""); err != nil {
			return Device{}, err
		}
		postgresTx = nil
		return s.deviceWithOwnerEmailLocked(existing), nil
	}
	now := time.Now().Unix()
	device := Device{
		DeviceID:   deviceID,
		Name:       strings.TrimSpace(name),
		Platform:   strings.TrimSpace(platform),
		OSName:     strings.TrimSpace(osName),
		OSVersion:  strings.TrimSpace(osVersion),
		Alias:      strings.TrimSpace(alias),
		PublicKey:  publicKey,
		GlobalName: sanitizeDNSLabel(deviceID) + "." + globalDeviceDomain(),
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	s.devices[device.DeviceID] = device
	s.runtimeStatuses[device.DeviceID] = DeviceRuntimeStatus{DeviceID: device.DeviceID, DeviceEnabled: true, LastReportAt: now}
	if err := s.persistPostgresDeviceRegisterTxLocked(ctx, postgresTx, device, NetworkDevice{}, ""); err != nil {
		return Device{}, err
	}
	postgresTx = nil
	return device, nil
}

func (s *Store) expirePreloginDevicesLocked(ctx context.Context, tx *sql.Tx, now int64) error {
	cutoff := now - int64(preloginDeviceTTL.Seconds())
	expired := make([]string, 0)
	for deviceID, device := range s.devices {
		if strings.TrimSpace(device.OwnerID) != "" {
			continue
		}
		if device.CreatedAt <= 0 || device.CreatedAt > cutoff {
			continue
		}
		expired = append(expired, deviceID)
	}
	for _, deviceID := range expired {
		if err := s.removeDeviceLocked(deviceID); err != nil {
			return err
		}
		if err := s.deletePostgresDeviceTxLocked(ctx, tx, deviceID); err != nil {
			return err
		}
	}
	return nil
}

func validPrepareDevicePublicKey(deviceID, publicKey string) bool {
	publicKey = strings.TrimSpace(publicKey)
	if publicKey == "" || publicKey == "client-v2-"+strings.TrimSpace(deviceID) {
		return false
	}
	if strings.HasPrefix(publicKey, "pk_") {
		token := strings.TrimPrefix(publicKey, "pk_")
		return len(token) >= 64 && asciiHex(token)
	}
	return len(publicKey) >= 32
}

func asciiHex(value string) bool {
	for _, ch := range value {
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return false
		}
	}
	return value != ""
}
