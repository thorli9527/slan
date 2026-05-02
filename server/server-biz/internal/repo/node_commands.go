package repo

import (
	"context"

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
		DoUpdates: clause.AssignmentColumns([]string{"observed_rtt_ms", "packet_loss_ppm", "path_score", "sampled_at_ms", "updated_at"}),
	}).Create(&record).Error
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

func (r *PostgresRepository) DeleteRelayNodeHeartbeatsBefore(ctx context.Context, cutoff int64) error {
	return r.db.WithContext(ctx).
		Where("updated_at < ?", cutoff).
		Delete(&RelayNodeHeartbeat{}).Error
}
