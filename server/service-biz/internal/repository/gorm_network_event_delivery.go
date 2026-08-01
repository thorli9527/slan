package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *GormStore) GetNetworkEventDelivery(ctx context.Context, eventID, targetDeviceID string) (model.NetworkEventDelivery, bool, error) {
	return firstModel(s.db.WithContext(ctx).Where("event_id = ? AND target_device_id = ?", eventID, targetDeviceID), func(row gormNetworkEventDeliveryRecord) model.NetworkEventDelivery { return row.model() })
}

func (s *GormStore) ClaimDueNetworkEventDeliveries(ctx context.Context, dueAt, leaseUntil int64, limit int) ([]model.NetworkEventDelivery, error) {
	if limit <= 0 {
		limit = 500
	}
	claimed := make([]model.NetworkEventDelivery, 0, limit)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []gormNetworkEventDeliveryRecord
		if err := tx.
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status = ? AND next_retry_at <= ?", "pending", dueAt).
			Order("next_retry_at asc").
			Limit(limit).
			Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			result := tx.Model(&gormNetworkEventDeliveryRecord{}).
				Where("event_id = ? AND target_device_id = ? AND status = ? AND next_retry_at <= ?", row.EventID, row.TargetDeviceID, "pending", dueAt).
				Update("next_retry_at", leaseUntil)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				continue
			}
			item := row.model()
			item.NextRetryAt = leaseUntil
			claimed = append(claimed, item)
		}
		return nil
	})
	return claimed, err
}

func (s *GormStore) ListDueNetworkEventDeliveries(ctx context.Context, dueAt int64, limit int) ([]model.NetworkEventDelivery, error) {
	if limit <= 0 {
		limit = 500
	}
	return listModels(s.db.WithContext(ctx).Where("status = ? AND next_retry_at <= ?", "pending", dueAt).Order("next_retry_at asc").Limit(limit), func(row gormNetworkEventDeliveryRecord) model.NetworkEventDelivery { return row.model() })
}

func (s *GormStore) SaveNetworkEventDelivery(ctx context.Context, item model.NetworkEventDelivery) error {
	row := networkEventDeliveryRecordFromModel(item)
	return s.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(&row).Error
}

func (s *GormStore) UpdatePendingNetworkEventDelivery(ctx context.Context, item model.NetworkEventDelivery) (bool, error) {
	row := networkEventDeliveryRecordFromModel(item)
	result := s.db.WithContext(ctx).
		Model(&gormNetworkEventDeliveryRecord{}).
		Where("event_id = ? AND target_device_id = ? AND status = ?", item.EventID, item.TargetDeviceID, "pending").
		Updates(map[string]any{
			"status":          row.Status,
			"attempts":        row.Attempts,
			"next_retry_at":   row.NextRetryAt,
			"acknowledged_at": row.AcknowledgedAt,
			"last_error":      row.LastError,
			"updated_at":      row.UpdatedAt,
		})
	return result.RowsAffected > 0, result.Error
}

func (s *GormStore) DeleteTerminalNetworkEventDeliveriesBefore(ctx context.Context, cutoff int64) (int64, error) {
	result := s.db.WithContext(ctx).
		Where("status <> ? AND updated_at < ?", "pending", cutoff).
		Delete(&gormNetworkEventDeliveryRecord{})
	return result.RowsAffected, result.Error
}

func networkEventDeliveryRecordFromModel(item model.NetworkEventDelivery) gormNetworkEventDeliveryRecord {
	return gormNetworkEventDeliveryRecord{
		EventID: item.EventID, TargetDeviceID: item.TargetDeviceID, NetworkID: item.NetworkID,
		EventType: item.EventType, ConfigVersion: item.ConfigVersion, Payload: item.Payload,
		Status: item.Status, Attempts: item.Attempts, NextRetryAt: item.NextRetryAt,
		ExpiresAt: item.ExpiresAt, AcknowledgedAt: item.AcknowledgedAt,
		LastError: item.LastError,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func (r gormNetworkEventDeliveryRecord) model() model.NetworkEventDelivery {
	return model.NetworkEventDelivery{
		EventID: r.EventID, TargetDeviceID: r.TargetDeviceID, NetworkID: r.NetworkID,
		EventType: r.EventType, ConfigVersion: r.ConfigVersion, Payload: r.Payload,
		Status: r.Status, Attempts: r.Attempts, NextRetryAt: r.NextRetryAt,
		ExpiresAt: r.ExpiresAt, AcknowledgedAt: r.AcknowledgedAt,
		LastError: r.LastError,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}
