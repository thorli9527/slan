package biz

import (
	"context"
	"sort"
	"strings"
	"time"
)

func (s *Store) ListOpsDevices() []OpsDeviceView {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]OpsDeviceView, 0, len(s.devices))
	for _, device := range s.devices {
		out = append(out, s.opsDeviceViewLocked(device))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt == out[j].UpdatedAt {
			return out[i].DeviceID < out[j].DeviceID
		}
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out
}

func (s *Store) UpdateOpsDevice(deviceID, alias, status string, enabled *bool) (OpsDeviceView, error) {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return OpsDeviceView{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return OpsDeviceView{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	device, ok := s.devices[deviceID]
	if !ok {
		return OpsDeviceView{}, errNotFound
	}
	if strings.TrimSpace(alias) != "" {
		device.Alias = strings.TrimSpace(alias)
	}
	if strings.TrimSpace(status) != "" {
		device.Status = strings.TrimSpace(status)
	}
	now := time.Now().Unix()
	device.UpdatedAt = now
	s.devices[deviceID] = device
	if enabled != nil {
		runtime := s.runtimeStatuses[deviceID]
		runtime.DeviceID = deviceID
		runtime.DeviceEnabled = *enabled
		runtime.LastReportAt = now
		if !*enabled {
			runtime.NetworkEnabled = false
		}
		s.runtimeStatuses[deviceID] = runtime
		for key, networkDevice := range s.networkDevices {
			if networkDevice.DeviceID != deviceID {
				continue
			}
			networkDevice.Enabled = *enabled
			if *enabled {
				networkDevice.Status = "active"
			} else {
				networkDevice.Status = "disabled"
			}
			networkDevice.UpdatedAt = now
			s.networkDevices[key] = networkDevice
		}
	}
	runtime := s.runtimeStatuses[deviceID]
	memberships := make([]NetworkDevice, 0)
	for _, networkDevice := range s.networkDevices {
		if networkDevice.DeviceID == deviceID {
			memberships = append(memberships, networkDevice)
		}
	}
	if err := s.persistPostgresOpsDeviceUpdateTxLocked(ctx, postgresTx, device, runtime, memberships); err != nil {
		return OpsDeviceView{}, err
	}
	postgresTx = nil
	return s.opsDeviceViewLocked(device), nil
}

func (s *Store) DeleteOpsDevice(deviceID string) error {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.removeDeviceLocked(deviceID)
}

func (s *Store) opsDeviceViewLocked(device Device) OpsDeviceView {
	ownerEmail := ""
	if user, ok := s.users[device.OwnerID]; ok {
		ownerEmail = user.Email
	}
	runtime := s.runtimeStatuses[device.DeviceID]
	networkCount := 0
	for _, networkDevice := range s.networkDevices {
		if networkDevice.DeviceID == device.DeviceID {
			networkCount++
		}
	}
	return OpsDeviceView{
		DeviceID:        device.DeviceID,
		OwnerID:         device.OwnerID,
		OwnerEmail:      ownerEmail,
		Name:            device.Name,
		Alias:           device.Alias,
		Platform:        device.Platform,
		OSName:          device.OSName,
		OSVersion:       device.OSVersion,
		GlobalIP:        device.GlobalIP,
		GlobalName:      device.GlobalName,
		Status:          device.Status,
		HeartbeatOnline: runtime.HeartbeatOnline,
		NetworkEnabled:  runtime.NetworkEnabled,
		DeviceEnabled:   runtime.DeviceEnabled,
		RxBytesTotal:    runtime.RxBytesTotal,
		TxBytesTotal:    runtime.TxBytesTotal,
		NetworkCount:    networkCount,
		LastSeenAt:      runtime.LastSeenAt,
		LastReportAt:    runtime.LastReportAt,
		CreatedAt:       device.CreatedAt,
		UpdatedAt:       device.UpdatedAt,
	}
}
