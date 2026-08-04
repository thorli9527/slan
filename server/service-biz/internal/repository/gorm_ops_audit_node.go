package repository

import (
	"context"

	"github.com/slan/service-biz/internal/model"
)

func (s *GormStore) ListAuditEvents(_ context.Context, limit int) ([]model.AuditEvent, error) {
	query := s.db.Order("created_at desc").Order("event_id desc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	return listModels(query, func(row gormAuditEventRecord) model.AuditEvent {
		return row.model()
	})
}

func (s *GormStore) SaveAuditEvent(_ context.Context, event model.AuditEvent) error {
	row := auditEventRecordFromModel(event)
	return upsertByColumns(s.db, &row, []string{"event_id"}, []string{"actor_type", "actor_id", "action", "resource_type", "resource_id", "status", "remote_ip", "detail", "created_at"})
}

func (s *GormStore) DeleteAuditEventsBefore(_ context.Context, cutoff int64) error {
	if cutoff <= 0 {
		return nil
	}
	return s.db.Where("created_at < ?", cutoff).Delete(&gormAuditEventRecord{}).Error
}

func (s *GormStore) ListRelayNodes(_ context.Context) ([]model.RelayNode, error) {
	return listModels(s.db.Order("node_id asc"), func(row gormRelayNodeRecord) model.RelayNode {
		return row.model()
	})
}

func (s *GormStore) GetRelayNode(_ context.Context, nodeID string) (model.RelayNode, bool, error) {
	return firstModel(s.db.Where("node_id = ?", nodeID), func(row gormRelayNodeRecord) model.RelayNode {
		return row.model()
	})
}

func (s *GormStore) SaveRelayNode(_ context.Context, item model.RelayNode) error {
	row := relayNodeRecordFromModel(item)
	return upsertByColumns(s.db, &row, []string{"node_id"}, []string{"name", "region", "endpoint", "transport", "priority", "ticket_key_source", "ticket_key_ring_id", "ticket_signing_configured", "ticket_key_ring_configured", "ticket_effective_key_count", "ticket_rotation_ready", "ticket_accepts_dev_fallback", "max_bandwidth_mbps", "monthly_traffic_gb", "used_traffic_gb", "max_sessions", "active_sessions", "status", "health", "created_at", "updated_at"})
}

func (s *GormStore) DeleteRelayNode(_ context.Context, nodeID string) error {
	return s.db.Delete(&gormRelayNodeRecord{}, "node_id = ?", nodeID).Error
}

func (s *GormStore) ListPunchNodes(_ context.Context) ([]model.PunchNode, error) {
	return listModels(s.db.Order("node_id asc"), func(row gormPunchNodeRecord) model.PunchNode {
		return row.model()
	})
}

func (s *GormStore) GetPunchNode(_ context.Context, nodeID string) (model.PunchNode, bool, error) {
	return firstModel(s.db.Where("node_id = ?", nodeID), func(row gormPunchNodeRecord) model.PunchNode {
		return row.model()
	})
}

func (s *GormStore) SavePunchNode(_ context.Context, item model.PunchNode) error {
	row := punchNodeRecordFromModel(item)
	return upsertByColumns(s.db, &row, []string{"node_id"}, []string{"name", "region", "endpoint", "max_sessions", "active_sessions", "status", "health", "priority", "created_at", "updated_at"})
}

func (s *GormStore) DeletePunchNode(_ context.Context, nodeID string) error {
	return s.db.Delete(&gormPunchNodeRecord{}, "node_id = ?", nodeID).Error
}
