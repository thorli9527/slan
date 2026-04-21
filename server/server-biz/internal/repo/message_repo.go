package repo

import (
	"context"

	"gorm.io/gorm"
)

func (r *PostgresRepository) CreateControlOutboundMessage(ctx context.Context, record ControlOutboundMessage) error {
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *PostgresRepository) GetControlOutboundMessage(ctx context.Context, messageID string) (ControlOutboundMessage, error) {
	var record ControlOutboundMessage
	err := r.db.WithContext(ctx).Where("message_id = ?", messageID).First(&record).Error
	return record, err
}

func (r *PostgresRepository) ListRetryableControlOutboundMessages(
	ctx context.Context,
	beforeAttemptAt int64,
	limit int,
) ([]ControlOutboundMessage, error) {
	var out []ControlOutboundMessage
	query := r.db.WithContext(ctx).
		Where("status = ? AND last_attempt_at <= ?", "pending", beforeAttemptAt).
		Order("last_attempt_at asc, created_at asc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&out).Error
	return out, err
}

func (r *PostgresRepository) UpdateControlOutboundMessageAttempt(
	ctx context.Context,
	messageID string,
	attemptCount int,
	attemptedAt int64,
) error {
	return r.db.WithContext(ctx).
		Model(&ControlOutboundMessage{}).
		Where("message_id = ? AND status = ?", messageID, "pending").
		Updates(map[string]any{
			"attempt_count":   attemptCount,
			"last_attempt_at": attemptedAt,
			"updated_at":      attemptedAt,
		}).Error
}

func (r *PostgresRepository) AckControlOutboundMessage(
	ctx context.Context,
	messageID string,
	targetUserID string,
	ackedAt int64,
) (ControlOutboundMessage, error) {
	var record ControlOutboundMessage
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("message_id = ? AND status = ?", messageID, "pending").First(&record).Error; err != nil {
			return err
		}
		if targetUserID != "" && record.TargetUserID != "" && record.TargetUserID != targetUserID {
			return gorm.ErrRecordNotFound
		}
		return tx.Model(&ControlOutboundMessage{}).
			Where("message_id = ? AND status = ?", messageID, "pending").
			Updates(map[string]any{
				"status":     "acked",
				"acked_at":   ackedAt,
				"updated_at": ackedAt,
			}).Error
	})
	return record, err
}

func (r *PostgresRepository) ArchiveControlOutboundMessage(
	ctx context.Context,
	record ControlOutboundMessage,
	finalStatus string,
	failureReason string,
	archivedAt int64,
) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&ControlMessageHistory{
			MessageID:     record.MessageID,
			TargetUserID:  record.TargetUserID,
			TargetNodeID:  record.TargetNodeID,
			NetworkID:     record.NetworkID,
			MessageType:   record.MessageType,
			RequestID:     record.RequestID,
			PayloadJSON:   record.PayloadJSON,
			AttemptCount:  record.AttemptCount,
			FinalStatus:   finalStatus,
			AckedAt:       record.AckedAt,
			ArchivedAt:    archivedAt,
			FailureReason: failureReason,
		}).Error; err != nil {
			return err
		}
		return tx.Where("message_id = ?", record.MessageID).Delete(&ControlOutboundMessage{}).Error
	})
}
