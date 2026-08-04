package service

import "context"

func (s OpsAuditService) ListAuditEvents(ctx context.Context, limit int) ([]OpsAuditEventView, error) {
	items, err := s.Audit.ListAuditEvents(ctx, normalizeAuditEventLimit(limit))
	if err != nil {
		return nil, err
	}
	out := make([]OpsAuditEventView, 0, len(items))
	for _, item := range items {
		out = append(out, opsAuditEventView(item))
	}
	return out, nil
}

func normalizeAuditEventLimit(limit int) int {
	if limit <= 0 {
		return 100
	}
	if limit > 500 {
		return 500
	}
	return limit
}
