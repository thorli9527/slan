package service

import "context"

type OpsDashboardUseCase interface {
	Dashboard(ctx context.Context) (OpsDashboardView, error)
}

type OpsAuditUseCase interface {
	ListAuditEvents(ctx context.Context, limit int) ([]OpsAuditEventView, error)
	RecordAuditEvent(ctx context.Context, input RecordOpsAuditEventInput) error
}

type RecordOpsAuditEventInput struct {
	ActorID, Action, ResourceType, ResourceID, Status string
}
