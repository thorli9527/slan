package repo

import (
	"context"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (r *PostgresRepository) UpsertNode(ctx context.Context, record Node) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "node_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "device_id", "node_public_key", "capabilities"}),
	}).Create(&record).Error
}

func (r *PostgresRepository) ReplaceNodeEndpoints(ctx context.Context, nodeID, networkID, natType string, endpoints []NodeEndpoint) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("node_id = ? AND network_id = ?", nodeID, networkID).Delete(&NodeEndpoint{}).Error; err != nil {
			return err
		}
		if len(endpoints) == 0 {
			return nil
		}
		for i := range endpoints {
			endpoints[i].NodeID = nodeID
			endpoints[i].NetworkID = networkID
			endpoints[i].NatType = natType
		}
		return tx.Create(&endpoints).Error
	})
}

func (r *PostgresRepository) UpsertNodeConnectionState(ctx context.Context, record NodeConnectionState) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "network_id"},
			{Name: "node_id"},
			{Name: "peer_node_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"path", "state", "reason", "observed_rtt_ms", "packet_loss_ppm", "path_score", "derp_node_id", "updated_at"}),
	}).Create(&record).Error
}

func (r *PostgresRepository) UpsertNodePathHealth(ctx context.Context, record NodePathHealth) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "network_id"},
			{Name: "node_id"},
			{Name: "peer_node_id"},
			{Name: "path_type"},
			{Name: "endpoint"},
			{Name: "derp_node_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"observed_rtt_ms",
			"packet_loss_ppm",
			"path_score",
			"source_country_code",
			"relay_country_code",
			"peer_country_code",
			"cross_country",
			"relay_mtu",
			"max_frame_payload",
			"sampled_at_ms",
			"updated_at",
		}),
	}).Create(&record).Error
}

func (r *PostgresRepository) InsertNodePathHealthSample(ctx context.Context, record NodePathHealthSample) error {
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *PostgresRepository) UpsertRelayNodeHeartbeat(ctx context.Context, record RelayNodeHeartbeat) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "node_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"cluster_id",
			"country_code",
			"city_code",
			"transport",
			"address",
			"healthy",
			"active_sessions",
			"reported_at_ms",
			"updated_at",
		}),
	}).Create(&record).Error
}

func (r *PostgresRepository) UpsertRelayPolicyExecution(ctx context.Context, record RelayPolicyExecution) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "network_id"},
			{Name: "device_id"},
			{Name: "policy_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"node_id",
			"template_id",
			"template_name",
			"scope",
			"relay_mtu",
			"max_frame_payload",
			"execution_level",
			"applied",
			"reason",
			"policy_updated_at_ms",
			"reported_at_ms",
			"updated_at",
		}),
	}).Create(&record).Error
}

func (r *PostgresRepository) DeleteRelayPolicyExecutions(ctx context.Context, networkID string, deviceIDs []string) error {
	if strings.TrimSpace(networkID) == "" || len(deviceIDs) == 0 {
		return nil
	}
	targets := make([]string, 0, len(deviceIDs))
	seen := make(map[string]bool, len(deviceIDs))
	for _, deviceID := range deviceIDs {
		deviceID = strings.TrimSpace(deviceID)
		if deviceID == "" || seen[deviceID] {
			continue
		}
		seen[deviceID] = true
		targets = append(targets, deviceID)
	}
	if len(targets) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Where("network_id = ? AND device_id IN ?", strings.TrimSpace(networkID), targets).
		Delete(&RelayPolicyExecution{}).Error
}

func (r *PostgresRepository) UpsertRelayPolicyTemplate(ctx context.Context, record RelayPolicyTemplate) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "template_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"network_id",
			"name",
			"description",
			"path_type",
			"path_strategy",
			"relay_mtu",
			"max_frame_payload",
			"recommendation_level",
			"execution_level",
			"ttl_minutes",
			"reason",
			"is_builtin",
			"updated_at",
		}),
	}).Create(&record).Error
}

func (r *PostgresRepository) CreateRelayPolicyTemplate(ctx context.Context, record RelayPolicyTemplate) error {
	return r.db.WithContext(ctx).Create(&record).Error
}

func (r *PostgresRepository) UpdateRelayPolicyTemplate(ctx context.Context, record RelayPolicyTemplate) error {
	result := r.db.WithContext(ctx).Model(&RelayPolicyTemplate{}).
		Where("template_id = ?", record.TemplateID).
		Updates(map[string]any{
			"network_id":           record.NetworkID,
			"name":                 record.Name,
			"description":          record.Description,
			"path_type":            record.PathType,
			"path_strategy":        record.PathStrategy,
			"relay_mtu":            record.RelayMtu,
			"max_frame_payload":    record.MaxFramePayload,
			"recommendation_level": record.RecommendationLevel,
			"execution_level":      record.ExecutionLevel,
			"ttl_minutes":          record.TTLMinutes,
			"reason":               record.Reason,
			"updated_at":           record.UpdatedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *PostgresRepository) DeleteRelayPolicyTemplate(ctx context.Context, templateID string) error {
	result := r.db.WithContext(ctx).Where("template_id = ? AND is_builtin = false", templateID).Delete(&RelayPolicyTemplate{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *PostgresRepository) DeleteNodePathHealthBefore(ctx context.Context, nodeID, networkID string, cutoff int64) error {
	return r.db.WithContext(ctx).
		Where("node_id = ? AND network_id = ? AND updated_at < ?", nodeID, networkID, cutoff).
		Delete(&NodePathHealth{}).Error
}

func (r *PostgresRepository) DeleteNodeEndpoints(ctx context.Context, nodeID, networkID string) error {
	return r.db.WithContext(ctx).
		Where("node_id = ? AND network_id = ?", nodeID, networkID).
		Delete(&NodeEndpoint{}).Error
}

func (r *PostgresRepository) DeleteNodeEndpointsBefore(ctx context.Context, nodeID, networkID string, cutoff int64) error {
	return r.db.WithContext(ctx).
		Where("node_id = ? AND network_id = ? AND updated_at < ?", nodeID, networkID, cutoff).
		Delete(&NodeEndpoint{}).Error
}

func (r *PostgresRepository) DeleteNodeEndpointsBeforeAll(ctx context.Context, cutoff int64) error {
	return r.db.WithContext(ctx).
		Where("updated_at < ?", cutoff).
		Delete(&NodeEndpoint{}).Error
}

func (r *PostgresRepository) DeleteNodeConnectionStates(ctx context.Context, nodeID, networkID string) error {
	return r.db.WithContext(ctx).
		Where("(node_id = ? OR peer_node_id = ?) AND network_id = ?", nodeID, nodeID, networkID).
		Delete(&NodeConnectionState{}).Error
}

func (r *PostgresRepository) DeleteNodeConnectionStatesBefore(ctx context.Context, nodeID, networkID string, cutoff int64) error {
	return r.db.WithContext(ctx).
		Where("(node_id = ? OR peer_node_id = ?) AND network_id = ? AND updated_at < ?", nodeID, nodeID, networkID, cutoff).
		Delete(&NodeConnectionState{}).Error
}

func (r *PostgresRepository) DeleteNodeConnectionStatesBeforeAll(ctx context.Context, cutoff int64) error {
	return r.db.WithContext(ctx).
		Where("updated_at < ?", cutoff).
		Delete(&NodeConnectionState{}).Error
}

func (r *PostgresRepository) DeleteNodePathHealthBeforeAll(ctx context.Context, cutoff int64) error {
	return r.db.WithContext(ctx).
		Where("updated_at < ?", cutoff).
		Delete(&NodePathHealth{}).Error
}

func (r *PostgresRepository) DeleteNodePathHealthSamplesBeforeAll(ctx context.Context, cutoffMs uint64) error {
	return r.db.WithContext(ctx).
		Where("sampled_at_ms < ?", cutoffMs).
		Delete(&NodePathHealthSample{}).Error
}

func (r *PostgresRepository) DeleteRelayNodeHeartbeatsBefore(ctx context.Context, cutoff int64) error {
	return r.db.WithContext(ctx).
		Where("updated_at < ?", cutoff).
		Delete(&RelayNodeHeartbeat{}).Error
}
