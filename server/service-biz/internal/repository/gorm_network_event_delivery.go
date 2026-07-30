package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
	"gorm.io/gorm/clause"
)

func (s *GormStore) GetNetworkEventDelivery(ctx context.Context, eventID, targetDeviceID string) (model.NetworkEventDelivery, bool, error) {
	return firstModel(s.db.WithContext(ctx).Where("event_id = ? AND target_device_id = ?", eventID, targetDeviceID), func(row gormNetworkEventDeliveryRecord) model.NetworkEventDelivery { return row.model() })
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

func networkEventDeliveryRecordFromModel(item model.NetworkEventDelivery) gormNetworkEventDeliveryRecord {
	return gormNetworkEventDeliveryRecord{
		EventID: item.EventID, TargetDeviceID: item.TargetDeviceID, NetworkID: item.NetworkID,
		EventType: item.EventType, ConfigVersion: item.ConfigVersion, Payload: item.Payload,
		Status: item.Status, Attempts: item.Attempts, NextRetryAt: item.NextRetryAt,
		ExpiresAt: item.ExpiresAt, AcknowledgedAt: item.AcknowledgedAt,
		CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}
}

func (r gormNetworkEventDeliveryRecord) model() model.NetworkEventDelivery {
	return model.NetworkEventDelivery{
		EventID: r.EventID, TargetDeviceID: r.TargetDeviceID, NetworkID: r.NetworkID,
		EventType: r.EventType, ConfigVersion: r.ConfigVersion, Payload: r.Payload,
		Status: r.Status, Attempts: r.Attempts, NextRetryAt: r.NextRetryAt,
		ExpiresAt: r.ExpiresAt, AcknowledgedAt: r.AcknowledgedAt,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}
