package service

import "github.com/slan/server/server-biz/internal/repo"

// MessageDelivery stores server-side outbound control message history.
type MessageDelivery interface {
	CreatePending(record repo.ControlOutboundMessage) error
	ListRetryable(beforeAttemptAt int64, limit int) ([]repo.ControlOutboundMessage, error)
	RecordAttempt(messageID string, attemptCount int, attemptedAt int64) error
	Archive(record repo.ControlOutboundMessage, finalStatus, failureReason string, archivedAt int64) error
}
