package biz

import (
	"context"
	"database/sql"
	"strings"
)

func (s *Store) persistPostgresControlDeliveryTxLocked(ctx context.Context, tx *sql.Tx, delivery MQTTControlDelivery) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	messageType := defaultString(strings.TrimSpace(delivery.MessageType), "control")
	expiresAt := delivery.ExpiresAt
	if expiresAt <= 0 {
		expiresAt = maxInt64(delivery.UpdatedAt+int64(defaultControlMessageTTLSeconds), delivery.AckedAt+int64(defaultControlMessageTTLSeconds))
	}
	createdAt := delivery.CreatedAt
	if createdAt <= 0 {
		createdAt = delivery.UpdatedAt
	}
	payloadJSON := string(delivery.Payload)
	if _, err := tx.ExecContext(ctx, `insert into mqtt_control_deliveries(id,delivery_id,device_id,message_type,task_id,action,status,attempt_count,last_error,payload,created_at,expires_at,published_at,processed_at,acked_at,updated_at)
		values($1,$2,$3,$4,$5,$6,$7,$8,$9,nullif($10,'')::jsonb,to_timestamp($11),to_timestamp($12),to_timestamp(nullif($13,0)),to_timestamp(nullif($14,0)),to_timestamp(nullif($15,0)),to_timestamp($16))
		on conflict(device_id, delivery_id) do update set
			message_type=excluded.message_type,
			task_id=excluded.task_id,
			action=excluded.action,
			status=excluded.status,
			attempt_count=excluded.attempt_count,
			last_error=excluded.last_error,
			payload=excluded.payload,
			created_at=excluded.created_at,
			expires_at=excluded.expires_at,
			published_at=excluded.published_at,
			processed_at=excluded.processed_at,
			acked_at=excluded.acked_at,
			updated_at=excluded.updated_at`,
		"mqtt-delivery-"+delivery.DeviceID+"-"+delivery.DeliveryID, delivery.DeliveryID, delivery.DeviceID, messageType, delivery.TaskID, delivery.Action, delivery.Status, delivery.AttemptCount, delivery.Error, payloadJSON, createdAt, expiresAt, delivery.PublishedAt, delivery.ProcessedAtMs/1000, delivery.AckedAt, delivery.UpdatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	return nil
}

func (s *Store) claimPostgresControlDeliveryRetryLocked(ctx context.Context, deviceID, deliveryID string, previousUpdatedAt, now int64) (bool, error) {
	result, err := s.db.ExecContext(ctx, `update mqtt_control_deliveries
		set status='retrying', updated_at=to_timestamp($1)
		where device_id=$2
			and delivery_id=$3
			and updated_at=to_timestamp($4)
			and status in ('pending','retrying','published')
			and acked_at is null
			and expires_at > to_timestamp($1)`,
		now, deviceID, deliveryID, previousUpdatedAt)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}
