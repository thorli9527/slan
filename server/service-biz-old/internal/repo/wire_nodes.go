package repo

import (
	"context"
	"time"

	"github.com/slan/server/server-biz/api/dto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WireDerpNode struct {
	RegionID            string `gorm:"column:region_id;primaryKey"`
	NodeID              string `gorm:"column:node_id;primaryKey"`
	Name                string `gorm:"column:name;not null;default:''"`
	Host                string `gorm:"column:host;not null"`
	Port                int    `gorm:"column:port;not null;default:443"`
	Enabled             bool   `gorm:"column:enabled;index;not null;default:true"`
	Healthy             bool   `gorm:"column:healthy;index;not null;default:true"`
	Priority            int    `gorm:"column:priority;index;not null;default:100"`
	UpdatedAtMs         int64  `gorm:"column:updated_at_ms;index;not null;default:0"`
	TicketKeySource     string `gorm:"column:ticket_key_source;not null;default:''"`
	TicketKeyRingID     string `gorm:"column:ticket_key_ring_id;index;not null;default:''"`
	TicketKeyCount      int    `gorm:"column:ticket_key_count;not null;default:0"`
	TicketRotationReady bool   `gorm:"column:ticket_rotation_ready;not null;default:false"`
}

func (WireDerpNode) TableName() string { return "wire_derp_nodes" }

type WireNodeListOptions struct {
	SchedulableOnly bool
	FreshAfterMs    int64
}

type WireNodeEventQuery struct {
	NodeKind      string
	RegionID      string
	NodeID        string
	EventType     string
	CreatedFromMs int64
	CreatedToMs   int64
	Limit         int
	Offset        int
}

type WireNodeEvent struct {
	EventID     uint64 `gorm:"column:event_id;primaryKey;autoIncrement"`
	NodeKind    string `gorm:"column:node_kind;index;not null"`
	RegionID    string `gorm:"column:region_id;index;not null"`
	NodeID      string `gorm:"column:node_id;index;not null"`
	EventType   string `gorm:"column:event_type;index;not null"`
	FromEnabled *bool  `gorm:"column:from_enabled"`
	ToEnabled   *bool  `gorm:"column:to_enabled"`
	FromHealthy *bool  `gorm:"column:from_healthy"`
	ToHealthy   *bool  `gorm:"column:to_healthy"`
	Reason      string `gorm:"column:reason;not null;default:''"`
	CreatedAtMs int64  `gorm:"column:created_at_ms;index;not null"`
}

func (WireNodeEvent) TableName() string { return "wire_node_events" }

func (m WireNodeEvent) ToDTO() dto.WireNodeEventRecord {
	return dto.WireNodeEventRecord{
		EventID:     m.EventID,
		NodeKind:    m.NodeKind,
		RegionID:    m.RegionID,
		NodeID:      m.NodeID,
		EventType:   m.EventType,
		FromEnabled: m.FromEnabled,
		ToEnabled:   m.ToEnabled,
		FromHealthy: m.FromHealthy,
		ToHealthy:   m.ToHealthy,
		Reason:      m.Reason,
		CreatedAtMs: m.CreatedAtMs,
	}
}

func (m WireDerpNode) ToDTO() dto.WireDerpNodeRecord {
	return dto.WireDerpNodeRecord{
		RegionID:    m.RegionID,
		NodeID:      m.NodeID,
		Name:        m.Name,
		Host:        m.Host,
		Port:        m.Port,
		Enabled:     m.Enabled,
		Healthy:     m.Healthy,
		Priority:    m.Priority,
		UpdatedAtMs: m.UpdatedAtMs,
		TicketKeyRotation: dto.WireTicketKeyStatus{
			Source:            m.TicketKeySource,
			KeyRingID:         m.TicketKeyRingID,
			EffectiveKeyCount: m.TicketKeyCount,
			KeyRingSize:       m.TicketKeyCount,
			RotationReady:     m.TicketRotationReady,
			Available:         m.TicketKeyRingID != "",
			ObservedAtMs:      m.UpdatedAtMs,
		},
	}
}

func (m WireDerpNode) ToDTOWithFreshness(freshAfterMs int64) dto.WireDerpNodeRecord {
	out := m.ToDTO()
	out.Stale = wireNodeIsStale(m.UpdatedAtMs, freshAfterMs)
	return out
}

type WireRelayNode struct {
	RegionID            string `gorm:"column:region_id;primaryKey"`
	NodeID              string `gorm:"column:node_id;primaryKey"`
	Host                string `gorm:"column:host;not null"`
	UDPPort             int    `gorm:"column:udp_port;not null;default:0"`
	AdminPort           int    `gorm:"column:admin_port;not null;default:0"`
	Enabled             bool   `gorm:"column:enabled;index;not null;default:true"`
	Healthy             bool   `gorm:"column:healthy;index;not null;default:true"`
	Priority            int    `gorm:"column:priority;index;not null;default:100"`
	UpdatedAtMs         int64  `gorm:"column:updated_at_ms;index;not null;default:0"`
	TicketKeySource     string `gorm:"column:ticket_key_source;not null;default:''"`
	TicketKeyRingID     string `gorm:"column:ticket_key_ring_id;index;not null;default:''"`
	TicketKeyCount      int    `gorm:"column:ticket_key_count;not null;default:0"`
	TicketRotationReady bool   `gorm:"column:ticket_rotation_ready;not null;default:false"`
}

func (WireRelayNode) TableName() string { return "wire_relay_nodes" }

func (m WireRelayNode) ToDTO() dto.WireRelayNodeRecord {
	return dto.WireRelayNodeRecord{
		RegionID:    m.RegionID,
		NodeID:      m.NodeID,
		Host:        m.Host,
		UDPPort:     m.UDPPort,
		AdminPort:   m.AdminPort,
		Enabled:     m.Enabled,
		Healthy:     m.Healthy,
		Priority:    m.Priority,
		UpdatedAtMs: m.UpdatedAtMs,
		TicketKeyRotation: dto.WireTicketKeyStatus{
			Source:            m.TicketKeySource,
			KeyRingID:         m.TicketKeyRingID,
			EffectiveKeyCount: m.TicketKeyCount,
			KeyRingSize:       m.TicketKeyCount,
			RotationReady:     m.TicketRotationReady,
			Available:         m.TicketKeyRingID != "",
			ObservedAtMs:      m.UpdatedAtMs,
		},
	}
}

func (m WireRelayNode) ToDTOWithFreshness(freshAfterMs int64) dto.WireRelayNodeRecord {
	out := m.ToDTO()
	out.Stale = wireNodeIsStale(m.UpdatedAtMs, freshAfterMs)
	return out
}

func wireNodeIsStale(updatedAtMs, freshAfterMs int64) bool {
	return freshAfterMs > 0 && updatedAtMs > 0 && updatedAtMs < freshAfterMs
}

func applyTicketKeyStatusUpdates(updates map[string]any, key dto.WireTicketKeyStatus) {
	if key.KeyRingID == "" && key.Source == "" && key.EffectiveKeyCount == 0 && key.KeyRingSize == 0 && !key.RotationReady {
		return
	}
	count := key.EffectiveKeyCount
	if count == 0 {
		count = key.KeyRingSize
	}
	updates["ticket_key_source"] = key.Source
	updates["ticket_key_ring_id"] = key.KeyRingID
	updates["ticket_key_count"] = count
	updates["ticket_rotation_ready"] = key.RotationReady
}

func (r *PostgresRepository) UpsertWireDerpNode(ctx context.Context, record WireDerpNode) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "region_id"}, {Name: "node_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name", "host", "port", "enabled", "healthy", "priority", "updated_at_ms",
			"ticket_key_source", "ticket_key_ring_id", "ticket_key_count", "ticket_rotation_ready",
		}),
	}).Create(&record).Error
}

func (r *PostgresRepository) UpdateWireDerpNodeHealth(ctx context.Context, regionID, nodeID string, healthy bool, updatedAtMs int64, key dto.WireTicketKeyStatus) error {
	updates := map[string]any{"healthy": healthy, "updated_at_ms": updatedAtMs}
	applyTicketKeyStatusUpdates(updates, key)
	tx := r.db.WithContext(ctx).Model(&WireDerpNode{}).
		Where("region_id = ? AND node_id = ?", regionID, nodeID).
		Updates(updates)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *PostgresRepository) UpdateWireDerpNodeStatus(ctx context.Context, regionID, nodeID string, updates map[string]any) error {
	tx := r.db.WithContext(ctx).Model(&WireDerpNode{}).
		Where("region_id = ? AND node_id = ?", regionID, nodeID).
		Updates(updates)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *PostgresRepository) GetWireDerpNode(ctx context.Context, regionID, nodeID string) (WireDerpNode, error) {
	var record WireDerpNode
	err := r.db.WithContext(ctx).Where("region_id = ? AND node_id = ?", regionID, nodeID).First(&record).Error
	return record, err
}

func (r *PostgresRepository) ListWireDerpNodes(ctx context.Context, opts WireNodeListOptions) ([]WireDerpNode, error) {
	var out []WireDerpNode
	q := r.db.WithContext(ctx)
	if opts.SchedulableOnly {
		q = q.Where("enabled = ? AND healthy = ?", true, true)
		if opts.FreshAfterMs > 0 {
			q = q.Where("updated_at_ms >= ?", opts.FreshAfterMs)
		}
	}
	err := q.Order("priority asc, region_id, node_id").Find(&out).Error
	return out, err
}

func (r *PostgresRepository) MarkStaleWireDerpNodesUnhealthy(ctx context.Context, staleBeforeMs int64) error {
	return r.db.WithContext(ctx).Model(&WireDerpNode{}).
		Where("healthy = ? AND updated_at_ms > 0 AND updated_at_ms < ?", true, staleBeforeMs).
		Updates(map[string]any{"healthy": false}).Error
}

func (r *PostgresRepository) ListStaleHealthyWireDerpNodes(ctx context.Context, staleBeforeMs int64) ([]WireDerpNode, error) {
	var out []WireDerpNode
	err := r.db.WithContext(ctx).
		Where("healthy = ? AND updated_at_ms > 0 AND updated_at_ms < ?", true, staleBeforeMs).
		Order("priority asc, region_id, node_id").
		Find(&out).Error
	return out, err
}

func (r *PostgresRepository) UpsertWireRelayNode(ctx context.Context, record WireRelayNode) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "region_id"}, {Name: "node_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"host", "udp_port", "admin_port", "enabled", "healthy", "priority", "updated_at_ms",
			"ticket_key_source", "ticket_key_ring_id", "ticket_key_count", "ticket_rotation_ready",
		}),
	}).Create(&record).Error
}

func (r *PostgresRepository) UpdateWireRelayNodeHealth(ctx context.Context, regionID, nodeID string, healthy bool, updatedAtMs int64, key dto.WireTicketKeyStatus) error {
	updates := map[string]any{"healthy": healthy, "updated_at_ms": updatedAtMs}
	applyTicketKeyStatusUpdates(updates, key)
	tx := r.db.WithContext(ctx).Model(&WireRelayNode{}).
		Where("region_id = ? AND node_id = ?", regionID, nodeID).
		Updates(updates)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *PostgresRepository) UpdateWireRelayNodeStatus(ctx context.Context, regionID, nodeID string, updates map[string]any) error {
	tx := r.db.WithContext(ctx).Model(&WireRelayNode{}).
		Where("region_id = ? AND node_id = ?", regionID, nodeID).
		Updates(updates)
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *PostgresRepository) GetWireRelayNode(ctx context.Context, regionID, nodeID string) (WireRelayNode, error) {
	var record WireRelayNode
	err := r.db.WithContext(ctx).Where("region_id = ? AND node_id = ?", regionID, nodeID).First(&record).Error
	return record, err
}

func (r *PostgresRepository) ListWireRelayNodes(ctx context.Context, opts WireNodeListOptions) ([]WireRelayNode, error) {
	var out []WireRelayNode
	q := r.db.WithContext(ctx)
	if opts.SchedulableOnly {
		q = q.Where("enabled = ? AND healthy = ?", true, true)
		if opts.FreshAfterMs > 0 {
			q = q.Where("updated_at_ms >= ?", opts.FreshAfterMs)
		}
	}
	err := q.Order("priority asc, region_id, node_id").Find(&out).Error
	return out, err
}

func (r *PostgresRepository) MarkStaleWireRelayNodesUnhealthy(ctx context.Context, staleBeforeMs int64) error {
	return r.db.WithContext(ctx).Model(&WireRelayNode{}).
		Where("healthy = ? AND updated_at_ms > 0 AND updated_at_ms < ?", true, staleBeforeMs).
		Updates(map[string]any{"healthy": false}).Error
}

func (r *PostgresRepository) ListStaleHealthyWireRelayNodes(ctx context.Context, staleBeforeMs int64) ([]WireRelayNode, error) {
	var out []WireRelayNode
	err := r.db.WithContext(ctx).
		Where("healthy = ? AND updated_at_ms > 0 AND updated_at_ms < ?", true, staleBeforeMs).
		Order("priority asc, region_id, node_id").
		Find(&out).Error
	return out, err
}

func (r *PostgresRepository) InsertWireNodeEvent(ctx context.Context, event WireNodeEvent) error {
	if event.CreatedAtMs == 0 {
		event.CreatedAtMs = time.Now().UnixMilli()
	}
	return r.db.WithContext(ctx).Create(&event).Error
}

func (r *PostgresRepository) ListRecentWireNodeEvents(ctx context.Context, limit int) ([]WireNodeEvent, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var out []WireNodeEvent
	err := r.db.WithContext(ctx).
		Order("created_at_ms desc, event_id desc").
		Limit(limit).
		Find(&out).Error
	return out, err
}

func (r *PostgresRepository) QueryWireNodeEvents(ctx context.Context, query WireNodeEventQuery) ([]WireNodeEvent, int64, error) {
	if query.Limit <= 0 || query.Limit > 200 {
		query.Limit = 50
	}
	if query.Offset < 0 {
		query.Offset = 0
	}
	q := r.db.WithContext(ctx).Model(&WireNodeEvent{})
	q = applyWireNodeEventFilters(q, query)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var out []WireNodeEvent
	err := applyWireNodeEventFilters(r.db.WithContext(ctx), query).
		Order("created_at_ms desc, event_id desc").
		Limit(query.Limit).
		Offset(query.Offset).
		Find(&out).Error
	return out, total, err
}

func applyWireNodeEventFilters(q *gorm.DB, query WireNodeEventQuery) *gorm.DB {
	if query.NodeKind != "" {
		q = q.Where("node_kind = ?", query.NodeKind)
	}
	if query.RegionID != "" {
		q = q.Where("region_id = ?", query.RegionID)
	}
	if query.NodeID != "" {
		q = q.Where("node_id = ?", query.NodeID)
	}
	if query.EventType != "" {
		q = q.Where("event_type = ?", query.EventType)
	}
	if query.CreatedFromMs > 0 {
		q = q.Where("created_at_ms >= ?", query.CreatedFromMs)
	}
	if query.CreatedToMs > 0 {
		q = q.Where("created_at_ms <= ?", query.CreatedToMs)
	}
	return q
}

func (r *PostgresRepository) DeleteWireNodeEventsBefore(ctx context.Context, cutoffMs int64) error {
	return r.db.WithContext(ctx).
		Where("created_at_ms > 0 AND created_at_ms < ?", cutoffMs).
		Delete(&WireNodeEvent{}).Error
}
