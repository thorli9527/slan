package impl

import (
	"context"

	"github.com/slan/server/server-biz/internal/repo"
	"github.com/slan/server/server-biz/internal/service"
)

type dbMessageDeliveryService struct{ state *dbState }

var _ service.MessageDelivery = dbMessageDeliveryService{}

func (s dbMessageDeliveryService) CreatePending(record repo.ControlOutboundMessage) error {
	return s.state.pg.CreateControlOutboundMessage(context.Background(), record)
}

func (s dbMessageDeliveryService) ListRetryable(beforeAttemptAt int64, limit int) ([]repo.ControlOutboundMessage, error) {
	return s.state.pg.ListRetryableControlOutboundMessages(context.Background(), beforeAttemptAt, limit)
}

func (s dbMessageDeliveryService) RecordAttempt(messageID string, attemptCount int, attemptedAt int64) error {
	return s.state.pg.UpdateControlOutboundMessageAttempt(context.Background(), messageID, attemptCount, attemptedAt)
}

func (s dbMessageDeliveryService) Archive(
	record repo.ControlOutboundMessage,
	finalStatus,
	failureReason string,
	archivedAt int64,
) error {
	return s.state.pg.ArchiveControlOutboundMessage(
		context.Background(),
		record,
		finalStatus,
		failureReason,
		archivedAt,
	)
}
