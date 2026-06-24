package service

import "context"

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
