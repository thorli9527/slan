package service

import "github.com/slan/server/server-biz/internal/repo"

// MessageDelivery stores server-side outbound control message history.
//
// Purpose:
//   - Provide persistence for "server -> device" messages when direct MQTT publish is not enough
//     (e.g., retries, dead-letter archive, observability).
//
// Storage model:
// - pending: messages waiting for delivery attempts.
// - retryable: messages that failed previously and are eligible for retry.
// - archived: messages that reached a terminal state.
type MessageDelivery interface {
	// CreatePending inserts a new outbound message record in pending state.
	CreatePending(record repo.ControlOutboundMessage) error

	// ListRetryable returns retryable outbound messages that should be attempted before beforeAttemptAt.
	//
	// Input:
	// - beforeAttemptAt: unix timestamp cutoff
	// - limit: max number of records
	ListRetryable(beforeAttemptAt int64, limit int) ([]repo.ControlOutboundMessage, error)

	// RecordAttempt records one delivery attempt, typically updating attempt count and last attempted time.
	RecordAttempt(messageID string, attemptCount int, attemptedAt int64) error

	// Archive moves a message to terminal archived state with final status and optional failure reason.
	Archive(record repo.ControlOutboundMessage, finalStatus, failureReason string, archivedAt int64) error
}
