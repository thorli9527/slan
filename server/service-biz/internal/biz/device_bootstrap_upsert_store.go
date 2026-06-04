package biz

import (
	"strings"
)

func (s *Store) upsertBootstrapDeviceLocked(ownerID, deviceID, name, platform, osName, osVersion, alias, publicKey string, now int64) Device {
	if existing, ok := s.devices[deviceID]; ok {
		if strings.TrimSpace(existing.OwnerID) == "" {
			s.addDeviceOwnerLocked(deviceID, ownerID, now)
			existing.OwnerID = ownerID
			if strings.TrimSpace(existing.GlobalIP) == "" {
				existing.GlobalIP = s.allocateGlobalIPLocked(deviceID, now)
			}
			existing.GlobalName = sanitizeDNSLabel(deviceID) + "." + globalDeviceDomain()
		} else if existing.OwnerID != ownerID {
			s.transferDeviceOwnerLocked(deviceID, existing.OwnerID, ownerID, "bootstrap_session_key", now)
			existing.OwnerID = ownerID
		}
		existing.Name = defaultString(strings.TrimSpace(name), existing.Name)
		existing.Platform = defaultString(strings.TrimSpace(platform), existing.Platform)
		existing.OSName = defaultString(strings.TrimSpace(osName), existing.OSName)
		existing.OSVersion = defaultString(strings.TrimSpace(osVersion), existing.OSVersion)
		existing.Alias = defaultString(strings.TrimSpace(alias), existing.Alias)
		existing.PublicKey = defaultString(strings.TrimSpace(publicKey), existing.PublicKey)
		existing.Status = "active"
		existing.UpdatedAt = now
		s.devices[deviceID] = existing
		return existing
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
	s.devices[deviceID] = device
	s.addDeviceOwnerLocked(device.DeviceID, ownerID, now)
	s.runtimeStatuses[device.DeviceID] = DeviceRuntimeStatus{DeviceID: device.DeviceID, DeviceEnabled: true, LastReportAt: now}
	return device
}
