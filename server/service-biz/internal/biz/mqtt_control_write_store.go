package biz

import (
	"context"
	"encoding/json"
	"strings"
)

func (s *Store) RecordMQTTControlAck(deviceID, deliveryID, taskID, action, status, errorText string, processedAtMs int64, now int64) (MQTTControlDelivery, error) {
	deviceID = strings.TrimSpace(deviceID)
	deliveryID = strings.TrimSpace(deliveryID)
	taskID = strings.TrimSpace(taskID)
	action = strings.TrimSpace(action)
	status = strings.TrimSpace(status)
	errorText = strings.TrimSpace(errorText)
	if deviceID == "" || deliveryID == "" || status == "" {
		return MQTTControlDelivery{}, errBadRequest
	}
	if status != "succeeded" && status != "failed" {
		return MQTTControlDelivery{}, errBadRequest
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return MQTTControlDelivery{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if _, ok := s.devices[deviceID]; !ok {
		return MQTTControlDelivery{}, errNotFound
	}
	delivery := s.controlDeliveries[controlDeliveryKey(deviceID, deliveryID)]
	if delivery.DeliveryID == "" {
		return MQTTControlDelivery{}, errNotFound
	}
	if delivery.AckedAt > 0 && (delivery.Status == "succeeded" || delivery.Status == "failed") {
		return delivery, nil
	}
	if delivery.Status == "expired" || (delivery.ExpiresAt > 0 && now >= delivery.ExpiresAt) {
		return MQTTControlDelivery{}, errConflict
	}
	delivery.DeliveryID = deliveryID
	delivery.DeviceID = deviceID
	delivery.TaskID = defaultString(taskID, delivery.TaskID)
	delivery.Action = defaultString(delivery.Action, action)
	delivery.Status = status
	delivery.Error = errorText
	delivery.ProcessedAtMs = processedAtMs
	delivery.AckedAt = now
	delivery.UpdatedAt = now
	if delivery.MessageType == "" {
		delivery.MessageType = "control_ack"
	}
	if delivery.ExpiresAt <= 0 {
		delivery.ExpiresAt = now + int64(defaultControlMessageTTLSeconds)
	}
	s.controlDeliveries[controlDeliveryKey(deviceID, deliveryID)] = delivery
	if err := s.persistControlDeliveriesLocked(); err != nil {
		return MQTTControlDelivery{}, err
	}
	if err := s.persistPostgresControlDeliveryTxLocked(ctx, postgresTx, delivery); err != nil {
		return MQTTControlDelivery{}, err
	}
	if postgresTx != nil {
		if err := postgresTx.Commit(); err != nil {
			return MQTTControlDelivery{}, err
		}
	}
	postgresTx = nil
	return delivery, nil
}

func (s *Store) PrepareMQTTControlDelivery(deviceID, deliveryID, messageType, action string, payload any, createdAt, expiresAt int64) (MQTTControlDelivery, error) {
	deviceID = strings.TrimSpace(deviceID)
	deliveryID = strings.TrimSpace(deliveryID)
	messageType = strings.TrimSpace(messageType)
	action = strings.TrimSpace(action)
	if deviceID == "" || deliveryID == "" || messageType == "" || expiresAt <= createdAt {
		return MQTTControlDelivery{}, errBadRequest
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return MQTTControlDelivery{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return MQTTControlDelivery{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	if _, ok := s.devices[deviceID]; !ok {
		return MQTTControlDelivery{}, errNotFound
	}
	delivery := s.controlDeliveries[controlDeliveryKey(deviceID, deliveryID)]
	if delivery.DeliveryID == "" {
		delivery = MQTTControlDelivery{
			DeliveryID: deliveryID,
			DeviceID:   deviceID,
			Status:     "pending",
			CreatedAt:  createdAt,
			ExpiresAt:  expiresAt,
			UpdatedAt:  createdAt,
		}
	}
	delivery.MessageType = messageType
	delivery.Action = action
	delivery.Payload = payloadJSON
	if delivery.CreatedAt <= 0 {
		delivery.CreatedAt = createdAt
	}
	if delivery.ExpiresAt <= 0 {
		delivery.ExpiresAt = expiresAt
	}
	if delivery.Status == "" {
		delivery.Status = "pending"
	}
	delivery.UpdatedAt = createdAt
	s.controlDeliveries[controlDeliveryKey(deviceID, deliveryID)] = delivery
	if err := s.persistControlDeliveriesLocked(); err != nil {
		return MQTTControlDelivery{}, err
	}
	if err := s.persistPostgresControlDeliveryTxLocked(ctx, postgresTx, delivery); err != nil {
		return MQTTControlDelivery{}, err
	}
	if postgresTx != nil {
		if err := postgresTx.Commit(); err != nil {
			return MQTTControlDelivery{}, err
		}
	}
	postgresTx = nil
	return delivery, nil
}

func (s *Store) RecordMQTTControlPublishResult(deviceID, deliveryID string, published bool, errorText string, now int64) (MQTTControlDelivery, error) {
	deviceID = strings.TrimSpace(deviceID)
	deliveryID = strings.TrimSpace(deliveryID)
	errorText = strings.TrimSpace(errorText)
	if deviceID == "" || deliveryID == "" {
		return MQTTControlDelivery{}, errBadRequest
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ctx := context.Background()
	postgresTx, err := s.beginPostgresCoreWriteLocked(ctx)
	if err != nil {
		return MQTTControlDelivery{}, err
	}
	defer rollbackPostgresCoreTx(postgresTx)
	delivery, ok := s.controlDeliveries[controlDeliveryKey(deviceID, deliveryID)]
	if !ok {
		return MQTTControlDelivery{}, errNotFound
	}
	delivery.AttemptCount++
	delivery.Error = errorText
	delivery.UpdatedAt = now
	if delivery.ExpiresAt > 0 && now >= delivery.ExpiresAt {
		delivery.Status = "expired"
	} else if published {
		delivery.Status = "published"
		delivery.PublishedAt = now
	} else {
		delivery.Status = "retrying"
	}
	s.controlDeliveries[controlDeliveryKey(deviceID, deliveryID)] = delivery
	if err := s.persistControlDeliveriesLocked(); err != nil {
		return MQTTControlDelivery{}, err
	}
	if err := s.persistPostgresControlDeliveryTxLocked(ctx, postgresTx, delivery); err != nil {
		return MQTTControlDelivery{}, err
	}
	if postgresTx != nil {
		if err := postgresTx.Commit(); err != nil {
			return MQTTControlDelivery{}, err
		}
	}
	postgresTx = nil
	return delivery, nil
}
