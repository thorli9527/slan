package biz

import (
	"context"
	"strings"
)

func (s *Store) GetMQTTControlDelivery(deliveryID string) (MQTTControlDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		delivery, ok, err := s.loadPostgresControlDeliveryByIDLocked(context.Background(), strings.TrimSpace(deliveryID))
		if err != nil {
			return MQTTControlDelivery{}, err
		}
		if ok {
			return logicalMQTTControlDeliveryState(delivery, timeNow().Unix()), nil
		}
	}
	delivery, ok := s.controlDeliveries[strings.TrimSpace(deliveryID)]
	if !ok {
		for _, delivery := range s.controlDeliveries {
			if delivery.DeliveryID == strings.TrimSpace(deliveryID) {
				return logicalMQTTControlDeliveryState(delivery, timeNow().Unix()), nil
			}
		}
		return MQTTControlDelivery{}, errNotFound
	}
	return logicalMQTTControlDeliveryState(delivery, timeNow().Unix()), nil
}

func (s *Store) GetMQTTControlDeliveryForDevice(deviceID, deliveryID string) (MQTTControlDelivery, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db != nil {
		delivery, ok, err := s.loadPostgresControlDeliveryForDeviceLocked(context.Background(), strings.TrimSpace(deviceID), strings.TrimSpace(deliveryID))
		if err != nil {
			return MQTTControlDelivery{}, err
		}
		if ok {
			return logicalMQTTControlDeliveryState(delivery, timeNow().Unix()), nil
		}
	}
	delivery, ok := s.controlDeliveries[controlDeliveryKey(deviceID, deliveryID)]
	if !ok {
		return MQTTControlDelivery{}, errNotFound
	}
	return logicalMQTTControlDeliveryState(delivery, timeNow().Unix()), nil
}

func controlDeliveryKey(deviceID, deliveryID string) string {
	return strings.TrimSpace(deviceID) + "\x00" + strings.TrimSpace(deliveryID)
}

func logicalMQTTControlDeliveryState(delivery MQTTControlDelivery, now int64) MQTTControlDelivery {
	if delivery.ExpiresAt <= 0 || now < delivery.ExpiresAt {
		return delivery
	}
	if delivery.Status != "pending" && delivery.Status != "retrying" && delivery.Status != "published" {
		return delivery
	}
	delivery.Status = "expired"
	return delivery
}
