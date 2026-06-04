package biz

import (
	"context"
	"fmt"
	"log"
	"sort"
)

func (s *Store) ListActiveNetworkDeviceIDs(networkID string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres active devices failed: %v", err)
	}
	out := make([]string, 0)
	for _, device := range s.networkDevices {
		if device.NetworkID == networkID && device.Enabled && device.Status == "active" {
			out = append(out, device.DeviceID)
		}
	}
	sort.Strings(out)
	return out
}

func (s *Store) HasActiveNetworkDevice(networkID, deviceID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres active device failed: %v", err)
		return false
	}
	device, ok := s.networkDevices[networkID+"|"+deviceID]
	return ok && device.Enabled && device.Status == "active"
}

func (s *Store) NextNetworkConfigVersion(networkID string, now int64) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	version := int64(1)
	for _, item := range s.configVersions {
		if item.NetworkID == networkID && item.ConfigVersion >= version {
			version = item.ConfigVersion + 1
		}
	}
	config := NetworkConfigVersion{
		ConfigID:      fmt.Sprintf("config-%06d", s.nextConfigSeq),
		NetworkID:     networkID,
		ConfigVersion: version,
		ConfigHash:    fmt.Sprintf("%s-%d", networkID, version),
		PushedAt:      now,
		Status:        "pushed",
	}
	s.nextConfigSeq++
	s.configVersions[config.ConfigID] = config
	return version
}

func (s *Store) currentNetworkConfigVersionLocked(networkID string) int64 {
	version := int64(1)
	for _, item := range s.configVersions {
		if item.NetworkID == networkID && item.ConfigVersion > version {
			version = item.ConfigVersion
		}
	}
	return version
}
