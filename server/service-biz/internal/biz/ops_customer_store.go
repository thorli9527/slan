package biz

import (
	"context"
	"sort"
	"strings"
	"time"
)

func (s *Store) ListCustomers() []CustomerProfile {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]CustomerProfile, 0, len(s.users))
	for _, user := range s.users {
		out = append(out, s.customerProfileLocked(user))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Email < out[j].Email })
	return out
}

func (s *Store) UpdateCustomerProfile(profile CustomerProfile) (CustomerProfile, error) {
	profile.CustomerID = strings.TrimSpace(profile.CustomerID)
	profile.Email = strings.ToLower(strings.TrimSpace(profile.Email))
	profile.Name = strings.TrimSpace(profile.Name)
	profile.Country = strings.TrimSpace(profile.Country)
	profile.Province = strings.TrimSpace(profile.Province)
	profile.City = strings.TrimSpace(profile.City)
	profile.IPRegion = strings.TrimSpace(profile.IPRegion)
	profile.Status = defaultString(profile.Status, "active")
	if profile.CustomerID == "" || profile.Email == "" {
		return CustomerProfile{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return CustomerProfile{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	user, ok := s.users[profile.CustomerID]
	if !ok {
		return CustomerProfile{}, errNotFound
	}
	if user.Email != profile.Email {
		if _, exists := s.userByEmail[profile.Email]; exists {
			return CustomerProfile{}, errConflict
		}
		delete(s.userByEmail, user.Email)
		user.Email = profile.Email
		s.userByEmail[user.Email] = user.UserID
	}
	user.Name = profile.Name
	now := time.Now().Unix()
	disableCustomer := profile.Status == "disabled"
	if profile.Status == "disabled" {
		user.Status = "disabled"
	} else {
		user.Status = "active"
	}
	user.UpdatedAt = now
	s.users[user.UserID] = user
	s.customerProfiles[user.UserID] = CustomerProfile{
		CustomerID: user.UserID,
		Country:    profile.Country,
		Province:   profile.Province,
		City:       profile.City,
		IPRegion:   profile.IPRegion,
		Status:     profile.Status,
	}
	affectedDevices := make([]Device, 0)
	affectedStatuses := make([]DeviceRuntimeStatus, 0)
	affectedMemberships := make([]NetworkDevice, 0)
	if disableCustomer {
		for token, session := range s.sessions {
			if session.UserID == user.UserID {
				delete(s.sessions, token)
			}
		}
		for sessionID, session := range s.deviceSessions {
			if session.UserID != user.UserID || session.State != "active" {
				continue
			}
			session.State = "revoked"
			s.deviceSessions[sessionID] = session
			delete(s.deviceSessionByToken, session.DeviceToken)
		}
		for deviceID, device := range s.devices {
			if device.OwnerID != user.UserID {
				continue
			}
			device.Status = "disabled"
			device.UpdatedAt = now
			s.devices[deviceID] = device
			status := s.runtimeStatuses[deviceID]
			status.DeviceID = deviceID
			status.DeviceEnabled = false
			status.NetworkEnabled = false
			status.LastReportAt = now
			s.runtimeStatuses[deviceID] = status
			affectedDevices = append(affectedDevices, device)
			affectedStatuses = append(affectedStatuses, status)
		}
		for key, networkDevice := range s.networkDevices {
			if networkDevice.OwnerUserID != user.UserID {
				continue
			}
			networkDevice.Enabled = false
			networkDevice.Status = "disabled"
			networkDevice.UpdatedAt = now
			s.networkDevices[key] = networkDevice
			affectedMemberships = append(affectedMemberships, networkDevice)
		}
	}
	if err := s.persistPostgresCustomerProfileUpdateTxLocked(ctx, postgresTx, user, disableCustomer, affectedDevices, affectedStatuses, affectedMemberships); err != nil {
		return CustomerProfile{}, err
	}
	postgresTx = nil
	return s.customerProfileLocked(user), nil
}
