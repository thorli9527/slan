package biz

// AuditService 负责审计事件的记录与查询编排。
type AuditService struct {
	store BusinessStore
}

func (s AuditService) Record(event AuditEvent) AuditEvent {
	return s.store.RecordAuditEvent(event)
}

func (s AuditService) List() []AuditEvent {
	return s.store.ListAuditEvents()
}

func (s AuditService) Query(filter AuditEventFilter) ([]AuditEvent, error) {
	return s.store.QueryAuditEvents(filter)
}
