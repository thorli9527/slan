package biz

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

type postgresControlDeliveryScanner interface {
	Scan(dest ...any) error
}

func (s *Store) loadPostgresControlDeliveriesLocked(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `select delivery_id,device_id,message_type,coalesce(task_id,''),coalesce(action,''),status,attempt_count,coalesce(last_error,''),coalesce(payload::text,''),extract(epoch from created_at)::bigint,extract(epoch from expires_at)::bigint,coalesce(extract(epoch from published_at)::bigint,0),coalesce(extract(epoch from processed_at)::bigint,0),coalesce(extract(epoch from acked_at)::bigint,0),extract(epoch from updated_at)::bigint from mqtt_control_deliveries`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var delivery MQTTControlDelivery
		var processedAtSeconds int64
		var payloadText string
		if err := rows.Scan(&delivery.DeliveryID, &delivery.DeviceID, &delivery.MessageType, &delivery.TaskID, &delivery.Action, &delivery.Status, &delivery.AttemptCount, &delivery.Error, &payloadText, &delivery.CreatedAt, &delivery.ExpiresAt, &delivery.PublishedAt, &processedAtSeconds, &delivery.AckedAt, &delivery.UpdatedAt); err != nil {
			return err
		}
		if strings.TrimSpace(payloadText) != "" {
			delivery.Payload = json.RawMessage(payloadText)
		}
		delivery.ProcessedAtMs = processedAtSeconds * 1000
		s.controlDeliveries[controlDeliveryKey(delivery.DeviceID, delivery.DeliveryID)] = delivery
	}
	return rows.Err()
}

func (s *Store) loadPostgresControlDeliveryByIDLocked(ctx context.Context, deliveryID string) (MQTTControlDelivery, bool, error) {
	if s.db == nil {
		return MQTTControlDelivery{}, false, nil
	}
	rows, err := s.db.QueryContext(ctx, `select delivery_id,device_id,message_type,coalesce(task_id,''),coalesce(action,''),status,attempt_count,coalesce(last_error,''),coalesce(payload::text,''),extract(epoch from created_at)::bigint,extract(epoch from expires_at)::bigint,coalesce(extract(epoch from published_at)::bigint,0),coalesce(extract(epoch from processed_at)::bigint,0),coalesce(extract(epoch from acked_at)::bigint,0),extract(epoch from updated_at)::bigint from mqtt_control_deliveries where delivery_id=$1 order by updated_at desc limit 1`, deliveryID)
	if err != nil {
		return MQTTControlDelivery{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return MQTTControlDelivery{}, false, rows.Err()
	}
	delivery, err := scanPostgresControlDelivery(rows)
	if err != nil {
		return MQTTControlDelivery{}, false, err
	}
	s.controlDeliveries[controlDeliveryKey(delivery.DeviceID, delivery.DeliveryID)] = delivery
	return delivery, true, rows.Err()
}

func (s *Store) loadPostgresControlDeliveryForDeviceLocked(ctx context.Context, deviceID, deliveryID string) (MQTTControlDelivery, bool, error) {
	if s.db == nil {
		return MQTTControlDelivery{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, `select delivery_id,device_id,message_type,coalesce(task_id,''),coalesce(action,''),status,attempt_count,coalesce(last_error,''),coalesce(payload::text,''),extract(epoch from created_at)::bigint,extract(epoch from expires_at)::bigint,coalesce(extract(epoch from published_at)::bigint,0),coalesce(extract(epoch from processed_at)::bigint,0),coalesce(extract(epoch from acked_at)::bigint,0),extract(epoch from updated_at)::bigint from mqtt_control_deliveries where device_id=$1 and delivery_id=$2`, deviceID, deliveryID)
	delivery, err := scanPostgresControlDelivery(row)
	if err == sql.ErrNoRows {
		return MQTTControlDelivery{}, false, nil
	}
	if err != nil {
		return MQTTControlDelivery{}, false, err
	}
	s.controlDeliveries[controlDeliveryKey(delivery.DeviceID, delivery.DeliveryID)] = delivery
	return delivery, true, nil
}

func scanPostgresControlDelivery(scanner postgresControlDeliveryScanner) (MQTTControlDelivery, error) {
	var delivery MQTTControlDelivery
	var processedAtSeconds int64
	var payloadText string
	if err := scanner.Scan(&delivery.DeliveryID, &delivery.DeviceID, &delivery.MessageType, &delivery.TaskID, &delivery.Action, &delivery.Status, &delivery.AttemptCount, &delivery.Error, &payloadText, &delivery.CreatedAt, &delivery.ExpiresAt, &delivery.PublishedAt, &processedAtSeconds, &delivery.AckedAt, &delivery.UpdatedAt); err != nil {
		return MQTTControlDelivery{}, err
	}
	if strings.TrimSpace(payloadText) != "" {
		delivery.Payload = json.RawMessage(payloadText)
	}
	delivery.ProcessedAtMs = processedAtSeconds * 1000
	return delivery, nil
}
