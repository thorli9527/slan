package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

type AuditRepository interface {
	ListAuditEvents(ctx context.Context, limit int) ([]model.AuditEvent, error)
	SaveAuditEvent(ctx context.Context, event model.AuditEvent) error
}
