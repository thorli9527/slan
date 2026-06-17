package biz

import (
	"context"
	"strings"
)

func (s *Store) ListDeviceGroups(userID string) []DeviceGroup {
	s.mu.Lock()
	defer s.mu.Unlock()
	userID = strings.TrimSpace(userID)
	out := make([]DeviceGroup, 0)
	for _, group := range s.deviceGroups {
		if userID == "" || group.UserID == userID {
			out = append(out, group)
		}
	}
	return sortedValuesFromSlice(out, func(a, b DeviceGroup) bool {
		if a.Name == b.Name {
			return a.GroupID < b.GroupID
		}
		return a.Name < b.Name
	})
}

func (s *Store) ListDeviceGroupMembers(userID string) []DeviceGroupMember {
	s.mu.Lock()
	defer s.mu.Unlock()
	userID = strings.TrimSpace(userID)
	out := make([]DeviceGroupMember, 0)
	for _, member := range s.deviceGroupMembers {
		group, ok := s.deviceGroups[member.GroupID]
		if !ok || (userID != "" && group.UserID != userID) {
			continue
		}
		out = append(out, member)
	}
	return sortedValuesFromSlice(out, func(a, b DeviceGroupMember) bool {
		if a.GroupID == b.GroupID {
			return a.DeviceID < b.DeviceID
		}
		return a.GroupID < b.GroupID
	})
}

func (s *Store) CreateDeviceGroup(userID, name string) (DeviceGroup, error) {
	userID = strings.TrimSpace(userID)
	name = strings.TrimSpace(name)
	if userID == "" || name == "" {
		return DeviceGroup{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	tx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return DeviceGroup{}, err
	}
	defer rollbackPostgresCoreTx(tx)
	if s.deviceGroupNameExistsLocked(userID, "", name) {
		return DeviceGroup{}, errConflict
	}
	now := timeNow().Unix()
	group := DeviceGroup{GroupID: newCompactUUID(), UserID: userID, Name: name, CreatedAt: now, UpdatedAt: now}
	s.deviceGroups[group.GroupID] = group
	s.nextDeviceGroupSeq++
	if err := s.persistPostgresDeviceGroupUpsertTxLocked(ctx, tx, group); err != nil {
		return DeviceGroup{}, err
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return DeviceGroup{}, err
		}
		tx = nil
	}
	return group, nil
}

func (s *Store) UpdateDeviceGroup(userID, groupID, name string) (DeviceGroup, error) {
	userID = strings.TrimSpace(userID)
	groupID = strings.TrimSpace(groupID)
	name = strings.TrimSpace(name)
	if userID == "" || groupID == "" || name == "" {
		return DeviceGroup{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	tx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return DeviceGroup{}, err
	}
	defer rollbackPostgresCoreTx(tx)
	group, ok := s.deviceGroups[groupID]
	if !ok || group.UserID != userID {
		return DeviceGroup{}, errNotFound
	}
	if s.deviceGroupNameExistsLocked(userID, groupID, name) {
		return DeviceGroup{}, errConflict
	}
	group.Name = name
	group.UpdatedAt = timeNow().Unix()
	s.deviceGroups[groupID] = group
	if err := s.persistPostgresDeviceGroupUpsertTxLocked(ctx, tx, group); err != nil {
		return DeviceGroup{}, err
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return DeviceGroup{}, err
		}
		tx = nil
	}
	return group, nil
}

func (s *Store) DeleteDeviceGroup(userID, groupID string) error {
	userID = strings.TrimSpace(userID)
	groupID = strings.TrimSpace(groupID)
	if userID == "" || groupID == "" {
		return errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	tx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(tx)
	group, ok := s.deviceGroups[groupID]
	if !ok || group.UserID != userID {
		return errNotFound
	}
	delete(s.deviceGroups, groupID)
	for key, member := range s.deviceGroupMembers {
		if member.GroupID == groupID {
			delete(s.deviceGroupMembers, key)
		}
	}
	if err := s.persistPostgresDeviceGroupDeleteTxLocked(ctx, tx, groupID); err != nil {
		return err
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return err
		}
		tx = nil
	}
	return nil
}

func (s *Store) SetDeviceGroups(userID, deviceID string, groupIDs []string) ([]DeviceGroupMember, error) {
	userID = strings.TrimSpace(userID)
	deviceID = strings.TrimSpace(deviceID)
	if userID == "" || deviceID == "" {
		return nil, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	tx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return nil, err
	}
	defer rollbackPostgresCoreTx(tx)
	if !s.deviceVisibleToUserLocked(deviceID, userID) {
		return nil, errNotFound
	}
	nextIDs := make(map[string]bool)
	for _, groupID := range groupIDs {
		groupID = strings.TrimSpace(groupID)
		if groupID == "" {
			continue
		}
		group, ok := s.deviceGroups[groupID]
		if !ok || group.UserID != userID {
			return nil, errBadRequest
		}
		nextIDs[groupID] = true
	}
	now := timeNow().Unix()
	for key, member := range s.deviceGroupMembers {
		if member.DeviceID == deviceID {
			group, ok := s.deviceGroups[member.GroupID]
			if ok && group.UserID == userID && !nextIDs[member.GroupID] {
				delete(s.deviceGroupMembers, key)
			}
		}
	}
	for groupID := range nextIDs {
		key := groupID + "|" + deviceID
		if _, ok := s.deviceGroupMembers[key]; !ok {
			s.deviceGroupMembers[key] = DeviceGroupMember{GroupID: groupID, DeviceID: deviceID, AddedAt: now}
		}
	}
	if err := s.persistPostgresDeviceGroupMembersForDeviceTxLocked(ctx, tx, userID, deviceID, s.deviceGroupMembersForDeviceLocked(userID, deviceID)); err != nil {
		return nil, err
	}
	if tx != nil {
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		tx = nil
	}
	return s.deviceGroupMembersForDeviceLocked(userID, deviceID), nil
}

func (s *Store) deviceGroupNameExistsLocked(userID, exceptGroupID, name string) bool {
	for _, group := range s.deviceGroups {
		if group.UserID == userID && group.GroupID != exceptGroupID && group.Name == name {
			return true
		}
	}
	return false
}

func (s *Store) deviceVisibleToUserLocked(deviceID, userID string) bool {
	if device, ok := s.devices[deviceID]; ok && device.OwnerID == userID {
		return true
	}
	if grant, ok := s.deviceAccessGrants[userID+"|"+deviceID]; ok && grant.Status == "active" {
		return true
	}
	return false
}

func (s *Store) deviceGroupMembersForDeviceLocked(userID, deviceID string) []DeviceGroupMember {
	out := make([]DeviceGroupMember, 0)
	for _, member := range s.deviceGroupMembers {
		group, ok := s.deviceGroups[member.GroupID]
		if ok && group.UserID == userID && member.DeviceID == deviceID {
			out = append(out, member)
		}
	}
	return sortedValuesFromSlice(out, func(a, b DeviceGroupMember) bool { return a.GroupID < b.GroupID })
}
