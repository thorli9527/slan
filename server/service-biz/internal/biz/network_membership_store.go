package biz

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"
)

func (s *Store) AddNetworkDevice(networkID, deviceID, actorUserID, alias string, enabled bool) (NetworkDevice, error) {
	actorUserID = strings.TrimSpace(actorUserID)
	if actorUserID == "" {
		return NetworkDevice{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return NetworkDevice{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if _, ok := s.networks[networkID]; !ok {
		return NetworkDevice{}, errNotFound
	}
	device, ok := s.devices[deviceID]
	if !ok {
		return NetworkDevice{}, errNotFound
	}
	if !s.canUserAddNetworkDeviceLocked(actorUserID, device.DeviceID, device.OwnerID) {
		return NetworkDevice{}, errBadRequest
	}
	if _, ok := s.networkDevices[networkID+"|"+deviceID]; ok {
		return NetworkDevice{}, errConflict
	}
	networkDevice := s.addNetworkDeviceLocked(networkID, deviceID, device.OwnerID, alias, enabled, time.Now().Unix())
	if err := s.persistPostgresNetworkDeviceUpsertTxLocked(ctx, postgresTx, networkDevice); err != nil {
		return NetworkDevice{}, err
	}
	postgresTx = nil
	return networkDevice, nil
}

func (s *Store) UpdateNetworkDevice(networkID, deviceID, alias string, enabled *bool) (NetworkDevice, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return NetworkDevice{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	key := networkID + "|" + deviceID
	networkDevice, ok := s.networkDevices[key]
	if !ok {
		return NetworkDevice{}, errNotFound
	}
	now := time.Now().Unix()
	networkDevice.Alias = strings.TrimSpace(alias)
	if enabled != nil {
		networkDevice.Enabled = *enabled
		if *enabled {
			networkDevice.Status = "active"
		} else {
			networkDevice.Status = "disabled"
		}
	}
	networkDevice.UpdatedAt = now
	s.networkDevices[key] = networkDevice
	if err := s.persistPostgresNetworkDeviceUpsertTxLocked(ctx, postgresTx, networkDevice); err != nil {
		return NetworkDevice{}, err
	}
	postgresTx = nil
	return networkDevice, nil
}

func (s *Store) RemoveNetworkDevice(networkID, deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	key := networkID + "|" + deviceID
	if _, ok := s.networkDevices[key]; !ok {
		return errNotFound
	}
	delete(s.networkDevices, key)
	if err := s.persistPostgresNetworkDeviceDeleteTxLocked(ctx, postgresTx, networkID, deviceID); err != nil {
		return err
	}
	postgresTx = nil
	return nil
}

func (s *Store) ListNetworkDevices(networkID string) []NetworkDevice {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres network devices failed: %v", err)
	}
	out := make([]NetworkDevice, 0)
	for _, device := range s.networkDevices {
		if networkID == "" || device.NetworkID == networkID {
			out = append(out, device)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DeviceID < out[j].DeviceID })
	return out
}

func (s *Store) ListNetworkDevicesForDevice(deviceID string) []NetworkDevice {
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres network devices for device failed: %v", err)
	}
	out := make([]NetworkDevice, 0)
	for _, device := range s.networkDevices {
		if device.DeviceID == deviceID {
			out = append(out, device)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].NetworkID == out[j].NetworkID {
			return out[i].NetworkDeviceID < out[j].NetworkDeviceID
		}
		return out[i].NetworkID < out[j].NetworkID
	})
	return out
}

func (s *Store) ListNetworkDevicesForUser(userID string) []NetworkDevice {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres network devices for user failed: %v", err)
	}
	out := make([]NetworkDevice, 0)
	for _, device := range s.networkDevices {
		if device.OwnerUserID == userID {
			out = append(out, device)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].NetworkID == out[j].NetworkID {
			if out[i].DeviceID == out[j].DeviceID {
				return out[i].NetworkDeviceID < out[j].NetworkDeviceID
			}
			return out[i].DeviceID < out[j].DeviceID
		}
		return out[i].NetworkID < out[j].NetworkID
	})
	return out
}
