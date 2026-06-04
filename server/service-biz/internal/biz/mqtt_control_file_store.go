package biz

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func controlDeliveryStorePathFromEnv() string {
	dir := strings.TrimSpace(os.Getenv("SLAN_BIZ_STATE_DIR"))
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "mqtt-control-deliveries.json")
}

func (s *Store) loadControlDeliveriesLocked() error {
	if strings.TrimSpace(s.controlDeliveryStorePath) == "" {
		return nil
	}
	payload, err := os.ReadFile(s.controlDeliveryStorePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var deliveries []MQTTControlDelivery
	if err := json.Unmarshal(payload, &deliveries); err != nil {
		return err
	}
	for _, delivery := range deliveries {
		if delivery.DeviceID == "" || delivery.DeliveryID == "" {
			continue
		}
		s.controlDeliveries[controlDeliveryKey(delivery.DeviceID, delivery.DeliveryID)] = delivery
	}
	return nil
}

func (s *Store) persistControlDeliveriesLocked() error {
	if strings.TrimSpace(s.controlDeliveryStorePath) == "" {
		return nil
	}
	deliveries := make([]MQTTControlDelivery, 0, len(s.controlDeliveries))
	for _, delivery := range s.controlDeliveries {
		deliveries = append(deliveries, delivery)
	}
	sort.Slice(deliveries, func(i, j int) bool {
		if deliveries[i].DeviceID == deliveries[j].DeviceID {
			return deliveries[i].DeliveryID < deliveries[j].DeliveryID
		}
		return deliveries[i].DeviceID < deliveries[j].DeviceID
	})
	payload, err := json.MarshalIndent(deliveries, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.controlDeliveryStorePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmpPath := s.controlDeliveryStorePath + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0o600); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.controlDeliveryStorePath)
}
