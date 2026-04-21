package service

import "github.com/slan/server/server-biz/internal/repo"

// MessageDelivery 定义服务端下行消息的 ACK、重试和归档能力。
type MessageDelivery interface {
	CreatePending(record repo.ControlOutboundMessage) error
	ListRetryable(beforeAttemptAt int64, limit int) ([]repo.ControlOutboundMessage, error)
	RecordAttempt(messageID string, attemptCount int, attemptedAt int64) error
	Ack(targetUserID, messageID string, ackedAt int64) error
	Archive(record repo.ControlOutboundMessage, finalStatus, failureReason string, archivedAt int64) error
}
