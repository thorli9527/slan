package biz

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"
)

func (s *Store) ListRetryableMQTTControlDeliveries(now int64, limit int) []MQTTControlDelivery {
	if limit <= 0 {
		limit = 100
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.refreshPostgresCoreLocked(context.Background()); err != nil {
		log.Printf("service-biz refresh postgres mqtt deliveries failed: %v", err)
	}
	out := make([]MQTTControlDelivery, 0)
	for _, delivery := range s.controlDeliveries {
		if delivery.Status != "pending" && delivery.Status != "retrying" && delivery.Status != "published" {
			continue
		}
		if delivery.ExpiresAt > 0 && now >= delivery.ExpiresAt {
			continue
		}
		if !mqttControlDeliveryDue(delivery, now) {
			continue
		}
		out = append(out, delivery)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt == out[j].UpdatedAt {
			return out[i].DeliveryID < out[j].DeliveryID
		}
		return out[i].UpdatedAt < out[j].UpdatedAt
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (s *Store) ClaimMQTTControlDeliveryRetry(deviceID, deliveryID string, previousUpdatedAt, now int64) (bool, error) {
	deviceID = strings.TrimSpace(deviceID)
	deliveryID = strings.TrimSpace(deliveryID)
	if deviceID == "" || deliveryID == "" || previousUpdatedAt <= 0 || now <= 0 {
		return false, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := controlDeliveryKey(deviceID, deliveryID)
	delivery, ok := s.controlDeliveries[key]
	if !ok {
		return false, nil
	}
	if delivery.UpdatedAt != previousUpdatedAt {
		return false, nil
	}
	if delivery.Status != "pending" && delivery.Status != "retrying" && delivery.Status != "published" {
		return false, nil
	}
	if delivery.ExpiresAt > 0 && now >= delivery.ExpiresAt {
		return false, s.expireMQTTControlDeliveryLocked(key, delivery, now)
	}
	if !mqttControlDeliveryDue(delivery, now) {
		return false, nil
	}
	if s.db != nil {
		claimed, err := s.claimPostgresControlDeliveryRetryLocked(context.Background(), deviceID, deliveryID, previousUpdatedAt, now)
		if err != nil || !claimed {
			return claimed, err
		}
	}
	delivery.Status = "retrying"
	delivery.UpdatedAt = now
	s.controlDeliveries[key] = delivery
	if err := s.persistControlDeliveriesLocked(); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) expireMQTTControlDeliveryLocked(key string, delivery MQTTControlDelivery, now int64) error {
	delivery.Status = "expired"
	delivery.UpdatedAt = now
	s.controlDeliveries[key] = delivery
	if err := s.persistControlDeliveriesLocked(); err != nil {
		return err
	}
	return s.persistPostgresExpiredControlDeliveriesLocked([]MQTTControlDelivery{delivery})
}

func (s *Store) persistPostgresExpiredControlDeliveriesLocked(deliveries []MQTTControlDelivery) error {
	if s.db == nil || len(deliveries) == 0 {
		return nil
	}
	ctx := context.Background()
	tx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return err
	}
	defer rollbackPostgresCoreTx(tx)
	for _, delivery := range deliveries {
		s.controlDeliveries[controlDeliveryKey(delivery.DeviceID, delivery.DeliveryID)] = delivery
		if err := s.persistPostgresControlDeliveryTxLocked(ctx, tx, delivery); err != nil {
			return err
		}
	}
	if tx != nil {
		return tx.Commit()
	}
	return nil
}

func (s *Store) ExpireMQTTControlDeliveries(now int64) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	expiredDeliveries := make([]MQTTControlDelivery, 0)
	for key, delivery := range s.controlDeliveries {
		if delivery.Status != "pending" && delivery.Status != "retrying" && delivery.Status != "published" {
			continue
		}
		if delivery.ExpiresAt <= 0 || now < delivery.ExpiresAt {
			continue
		}
		delivery.Status = "expired"
		delivery.UpdatedAt = now
		s.controlDeliveries[key] = delivery
		expiredDeliveries = append(expiredDeliveries, delivery)
	}
	if len(expiredDeliveries) == 0 {
		return 0
	}
	if err := s.persistControlDeliveriesLocked(); err != nil {
		log.Printf("service-biz expire mqtt deliveries local persist failed: %v", err)
	}
	if err := s.persistPostgresExpiredControlDeliveriesLocked(expiredDeliveries); err != nil {
		log.Printf("service-biz expire mqtt deliveries postgres persist failed: %v", err)
	}
	return len(expiredDeliveries)
}

func mqttControlDeliveryDue(delivery MQTTControlDelivery, now int64) bool {
	if delivery.AttemptCount <= 0 {
		return true
	}
	return now >= delivery.UpdatedAt+int64(mqttControlRetryDelay(delivery.AttemptCount).Seconds())
}

func mqttControlRetryDelay(attemptCount int) time.Duration {
	switch {
	case attemptCount <= 1:
		return time.Second
	case attemptCount == 2:
		return 3 * time.Second
	case attemptCount == 3:
		return 6 * time.Second
	default:
		return 15 * time.Second
	}
}
