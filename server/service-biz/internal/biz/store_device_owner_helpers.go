package biz

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (s *Store) addDeviceOwnerLocked(deviceID, userID string, now int64) DeviceOwner {
	owner := DeviceOwner{OwnerRecordID: fmt.Sprintf("owner-%06d", s.nextOwnerSeq), DeviceID: deviceID, UserID: userID, Status: "active", BoundAt: now}
	s.nextOwnerSeq++
	s.deviceOwners[deviceID] = owner
	return owner
}

func (s *Store) transferDeviceOwnerLocked(deviceID, fromUserID, toUserID, reason string, now int64) {
	owner := s.deviceOwners[deviceID]
	owner.Status = "transferred"
	owner.UnboundAt = now
	s.deviceOwners[deviceID+"|old|"+fmt.Sprint(now)] = owner
	s.addDeviceOwnerLocked(deviceID, toUserID, now)
	log := DeviceOwnerChangeLog{LogID: fmt.Sprintf("owner-log-%06d", s.nextOwnerLogSeq), DeviceID: deviceID, FromUserID: fromUserID, ToUserID: toUserID, Reason: reason, ChangedAt: now}
	s.nextOwnerLogSeq++
	s.ownerLogs[log.LogID] = log
}

func (s *Store) bindExistingDeviceToUser(deviceID, userID string) error {
	deviceID = strings.TrimSpace(deviceID)
	userID = strings.TrimSpace(userID)
	if deviceID == "" || userID == "" {
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
	if _, ok := s.users[userID]; !ok {
		return errNotFound
	}
	now := time.Now().Unix()
	previousOwnerID := ""
	if strings.TrimSpace(device.OwnerID) == "" {
		device.OwnerID = userID
		if strings.TrimSpace(device.GlobalIP) == "" {
			device.GlobalIP = s.allocateGlobalIPLocked(deviceID, now)
		}
		device.GlobalName = sanitizeDNSLabel(deviceID) + "." + globalDeviceDomain()
		device.UpdatedAt = now
		s.devices[deviceID] = device
		s.addDeviceOwnerLocked(deviceID, userID, now)
	} else if device.OwnerID != userID {
		previousOwnerID = device.OwnerID
		s.transferDeviceOwnerLocked(deviceID, device.OwnerID, userID, "device_login_complete", now)
		s.removeDeviceFromUserNetworksLocked(deviceID, device.OwnerID)
		s.revokeDeviceSessionsLocked(deviceID, device.OwnerID)
		device.OwnerID = userID
		device.Status = "active"
		device.UpdatedAt = now
		s.devices[deviceID] = device
	}
	network := s.ensureDefaultNetworkForUserLocked(userID, now)
	membership := NetworkDevice{
		NetworkDeviceID: newCompactUUID(),
		NetworkID:       network.NetworkID,
		DeviceID:        deviceID,
		OwnerUserID:     userID,
		Alias:           device.Alias,
		Enabled:         true,
		Status:          "active",
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	s.networkDevices[network.NetworkID+"|"+deviceID] = membership
	if err := s.persistPostgresDeviceRegisterTxLocked(ctx, postgresTx, device, membership, previousOwnerID); err != nil {
		return err
	}
	postgresTx = nil
	return nil
}

func (s *Store) canUserAddNetworkDeviceLocked(userID, deviceID, ownerUserID string) bool {
	return s.canUserSeeDeviceLocked(userID, deviceID, ownerUserID)
}

func (s *Store) canUserSeeDeviceLocked(userID, deviceID, ownerUserID string) bool {
	if userID == "" {
		return false
	}
	if ownerUserID == userID {
		return true
	}
	if grant, ok := s.deviceAccessGrants[userID+"|"+deviceID]; ok && grant.Status == "active" {
		return true
	}
	return false
}

func (s *Store) deviceWithOwnerEmailLocked(device Device) Device {
	if user, ok := s.users[device.OwnerID]; ok {
		device.OwnerEmail = user.Email
	}
	return s.deviceWithSubnetLocked(device)
}

func (s *Store) deviceWithSubnetLocked(device Device) Device {
	globalIP := hostIP(device.GlobalIP)
	if globalIP == "" {
		return device
	}
	device.GlobalIP = globalIP
	subnet, ok := s.globalIPSubnetLocked(globalIP)
	if !ok {
		return device
	}
	device.PrefixLen = subnet.PrefixLength
	device.GlobalCIDR = ipamGlobalCIDR
	device.SubnetID = subnet.SubnetID
	device.SubnetCIDR = subnet.CIDRBlock
	device.SubnetPrefixLen = subnet.PrefixLength
	return device
}
