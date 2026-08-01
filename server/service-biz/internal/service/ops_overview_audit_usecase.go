package service

import "context"

import "github.com/slan/service-biz/internal/model"

func (s OpsAuditService) ListAuditEvents(ctx context.Context, limit int) ([]OpsAuditEventView, error) {
	items, err := s.Audit.ListAuditEvents(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]OpsAuditEventView, 0, len(items))
	for _, item := range items {
		out = append(out, opsAuditEventView(item))
	}
	return out, nil
}

func (s OpsAuditService) RecordAuditEvent(ctx context.Context, input RecordOpsAuditEventInput) error {
	if s.Audit == nil {
		return nil
	}
	now := opsNow(s.Now)
	suffix, err := randomHex(4)
	if err != nil {
		return err
	}
	return s.Audit.SaveAuditEvent(ctx, model.AuditEvent{
		EventID:   "audit_" + now.Format("20060102150405.000000000") + "_" + suffix,
		ActorType: "operator", ActorID: input.ActorID, Action: input.Action,
		ResourceType: input.ResourceType, ResourceID: input.ResourceID,
		Status: firstNonEmpty(input.Status, "success"), CreatedAt: now.Unix(),
	})
}
