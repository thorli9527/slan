package biz

import (
	"context"
	"strings"
	"time"
)

func (s *Store) RegisterDevice(ownerID, deviceID, name, platform, osName, osVersion, alias, publicKey string) (Device, NetworkDevice, error) {
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return Device{}, NetworkDevice{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return Device{}, NetworkDevice{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if _, ok := s.users[ownerID]; !ok {
		return Device{}, NetworkDevice{}, errNotFound
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		deviceID = newCompactUUID()
		s.nextDeviceSeq++
	}
	now := time.Now().Unix()
	if existing, ok := s.devices[deviceID]; ok {
		if strings.TrimSpace(existing.OwnerID) == "" {
			existing.OwnerID = ownerID
			if strings.TrimSpace(existing.GlobalIP) == "" {
				existing.GlobalIP = s.allocateGlobalIPLocked(deviceID, now)
			}
			existing.GlobalName = sanitizeDNSLabel(deviceID) + "." + globalDeviceDomain()
			existing.Name = defaultString(strings.TrimSpace(name), existing.Name)
			existing.Platform = defaultString(strings.TrimSpace(platform), existing.Platform)
			existing.OSName = defaultString(strings.TrimSpace(osName), existing.OSName)
			existing.OSVersion = defaultString(strings.TrimSpace(osVersion), existing.OSVersion)
			existing.Alias = defaultString(strings.TrimSpace(alias), existing.Alias)
			existing.PublicKey = defaultString(strings.TrimSpace(publicKey), existing.PublicKey)
			existing.Status = "active"
			existing.UpdatedAt = now
			s.devices[deviceID] = existing
			s.addDeviceOwnerLocked(deviceID, ownerID, now)
			defaultNetwork := s.ensureDefaultNetworkForUserLocked(ownerID, now)
			networkDevice := s.addNetworkDeviceLocked(defaultNetwork.NetworkID, deviceID, ownerID, alias, true, now)
			s.runtimeStatuses[deviceID] = DeviceRuntimeStatus{DeviceID: deviceID, DeviceEnabled: true, LastReportAt: now}
			if err := s.persistPostgresDeviceRegisterTxLocked(ctx, postgresTx, existing, networkDevice, ""); err != nil {
				return Device{}, NetworkDevice{}, err
			}
			postgresTx = nil
			return s.deviceWithOwnerEmailLocked(existing), networkDevice, nil
		}
		if existing.OwnerID != ownerID {
			return Device{}, NetworkDevice{}, errConflict
		}
		return Device{}, NetworkDevice{}, errConflict
	}
	ip := s.allocateGlobalIPLocked(deviceID, now)
	device := Device{
		DeviceID:   deviceID,
		OwnerID:    ownerID,
		Name:       strings.TrimSpace(name),
		Platform:   strings.TrimSpace(platform),
		OSName:     strings.TrimSpace(osName),
		OSVersion:  strings.TrimSpace(osVersion),
		Alias:      strings.TrimSpace(alias),
		PublicKey:  strings.TrimSpace(publicKey),
		GlobalIP:   ip,
		GlobalName: sanitizeDNSLabel(deviceID) + "." + globalDeviceDomain(),
		Status:     "active",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	s.devices[device.DeviceID] = device
	s.addDeviceOwnerLocked(device.DeviceID, ownerID, now)
	defaultNetwork := s.ensureDefaultNetworkForUserLocked(ownerID, now)
	networkDevice := s.addNetworkDeviceLocked(defaultNetwork.NetworkID, device.DeviceID, ownerID, alias, true, now)
	s.runtimeStatuses[device.DeviceID] = DeviceRuntimeStatus{DeviceID: device.DeviceID, DeviceEnabled: true, LastReportAt: now}
	if err := s.persistPostgresDeviceRegisterTxLocked(ctx, postgresTx, device, networkDevice, ""); err != nil {
		return Device{}, NetworkDevice{}, err
	}
	postgresTx = nil
	return s.deviceWithOwnerEmailLocked(device), networkDevice, nil
}
