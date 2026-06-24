package service

import "context"

type OpsDashboardUseCase interface {
	Dashboard(ctx context.Context) (OpsDashboardView, error)
}

type OpsAuditUseCase interface {
	ListAuditEvents(ctx context.Context, limit int) ([]OpsAuditEventView, error)
}
