package biz

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"
)

func (s *Store) ListUsers() []User {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres users failed: %v", err)
	}
	return sortedValues(s.users, func(a, b User) bool { return a.Email < b.Email })
}

func (s *Store) ListDevices(userID string) []Device {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres devices failed: %v", err)
	}
	out := make([]Device, 0)
	for _, device := range s.devices {
		if userID == "" || device.OwnerID == userID {
			out = append(out, s.deviceWithOwnerEmailLocked(device))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeviceID < out[j].DeviceID })
	return out
}

func (s *Store) RenewDevice(deviceID, userID string, networkEnabled bool, rxBytesTotal, txBytesTotal uint64) (Device, []NetworkConfig, error) {
	deviceID = strings.TrimSpace(deviceID)
	userID = strings.TrimSpace(userID)
	if deviceID == "" || userID == "" {
		return Device{}, nil, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return Device{}, nil, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	device, ok := s.devices[deviceID]
	if !ok {
		return Device{}, nil, errNotFound
	}
	if !s.canUserSeeDeviceLocked(userID, deviceID, device.OwnerID) {
		return Device{}, nil, errNotFound
	}
	now := time.Now().Unix()
	device.Status = "active"
	device.UpdatedAt = now
	s.devices[deviceID] = device
	status := s.runtimeStatuses[deviceID]
	status.DeviceID = deviceID
	status.HeartbeatOnline = true
	status.NetworkEnabled = networkEnabled
	status.DeviceEnabled = true
	status.RxBytesTotal = rxBytesTotal
	status.TxBytesTotal = txBytesTotal
	status.LastSeenAt = now
	status.LastReportAt = now
	s.runtimeStatuses[deviceID] = status
	configs, err := s.networkConfigsForDeviceLocked(deviceID)
	if err != nil {
		return Device{}, nil, err
	}
	if err := s.persistPostgresDeviceRuntimeTxLocked(ctx, postgresTx, device, status); err != nil {
		return Device{}, nil, err
	}
	postgresTx = nil
	return s.deviceWithOwnerEmailLocked(device), configs, nil
}

func (s *Store) ReportDeviceRuntime(deviceID string, networkEnabled bool, rxBytesTotal, txBytesTotal uint64) DeviceRuntimeReportResult {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return DeviceRuntimeReportResult{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		log.Printf("service-biz prepare runtime status write failed device=%s: %v", deviceID, err)
		return DeviceRuntimeReportResult{}
	}
	defer rollbackPostgresCoreTx(postgresTx)
	device, ok := s.devices[deviceID]
	if !ok {
		return DeviceRuntimeReportResult{}
	}
	now := time.Now().Unix()
	previous := s.runtimeStatuses[deviceID]
	status := s.runtimeStatuses[deviceID]
	status.DeviceID = deviceID
	status.HeartbeatOnline = true
	status.NetworkEnabled = networkEnabled
	status.DeviceEnabled = device.Status == "active"
	status.RxBytesTotal = rxBytesTotal
	status.TxBytesTotal = txBytesTotal
	status.LastSeenAt = now
	status.LastReportAt = now
	s.runtimeStatuses[deviceID] = status
	if err := s.persistPostgresDeviceRuntimeTxLocked(ctx, postgresTx, device, status); err != nil {
		log.Printf("service-biz persist runtime status failed device=%s: %v", deviceID, err)
		return DeviceRuntimeReportResult{}
	}
	postgresTx = nil
	networkIDs := make([]string, 0)
	for _, membership := range s.networkDevices {
		if membership.DeviceID == deviceID && membership.Enabled && membership.Status == "active" {
			networkIDs = append(networkIDs, membership.NetworkID)
		}
	}
	sort.Strings(networkIDs)
	return DeviceRuntimeReportResult{
		DeviceID:              deviceID,
		NetworkEnabled:        networkEnabled,
		NetworkEnabledChanged: previous.LastReportAt == 0 || previous.NetworkEnabled != networkEnabled,
		NetworkIDs:            networkIDs,
		ChangedAt:             now,
	}
}

func (s *Store) ReportDeviceEndpoint(networkID, deviceID string, endpoints []DeviceEndpoint) (bool, error) {
	networkID = strings.TrimSpace(networkID)
	deviceID = strings.TrimSpace(deviceID)
	if networkID == "" || deviceID == "" {
		return false, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.networkDevices[networkID+"|"+deviceID]; !ok {
		return false, errNotFound
	}
	now := time.Now().Unix()
	cleaned := make([]DeviceEndpoint, 0, len(endpoints))
	seen := make(map[string]bool)
	for _, endpoint := range endpoints {
		endpoint.Type = strings.TrimSpace(endpoint.Type)
		endpoint.Address = strings.TrimSpace(endpoint.Address)
		if endpoint.Type == "" {
			endpoint.Type = "direct_udp"
		}
		if endpoint.Address == "" || strings.Contains(endpoint.Address, "://") {
			continue
		}
		if endpoint.UpdatedAt == 0 {
			endpoint.UpdatedAt = now
		}
		key := endpoint.Type + "|" + endpoint.Address
		if seen[key] {
			continue
		}
		seen[key] = true
		cleaned = append(cleaned, endpoint)
	}
	key := networkID + "|" + deviceID
	previous := s.deviceEndpoints[key]
	if len(cleaned) == 0 {
		delete(s.deviceEndpoints, key)
		return len(previous) > 0, nil
	}
	s.deviceEndpoints[key] = cleaned
	return deviceEndpointsChanged(previous, cleaned), nil
}

func deviceEndpointsChanged(previous, next []DeviceEndpoint) bool {
	if len(previous) != len(next) {
		return true
	}
	for index := range previous {
		if previous[index].Type != next[index].Type || previous[index].Address != next[index].Address {
			return true
		}
	}
	return false
}
