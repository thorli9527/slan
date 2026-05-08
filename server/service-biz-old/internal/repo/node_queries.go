package repo

import (
	"context"
)

// GetNodeByID loads one node row by its stable public id.
func (r *PostgresRepository) GetNodeByID(ctx context.Context, nodeID string) (Node, error) {
	var record Node
	err := r.db.WithContext(ctx).Where("node_id = ?", nodeID).First(&record).Error
	return record, err
}

// ListNodesByNetwork returns the distinct nodes whose backing devices are
// active members of the target network.
func (r *PostgresRepository) ListNodesByNetwork(ctx context.Context, networkID string) ([]Node, error) {
	var out []Node
	err := r.db.WithContext(ctx).
		Table("nodes").
		Select("distinct nodes.*").
		Joins("join network_members on network_members.device_id = nodes.device_id").
		Where("network_members.network_id = ? AND network_members.status = ?", networkID, "active").
		Order("nodes.node_id").
		Find(&out).Error
	return out, err
}

// ListNodeEndpoints returns the node's advertised endpoints for one network,
// newest first.
func (r *PostgresRepository) ListNodeEndpoints(ctx context.Context, nodeID, networkID string) ([]NodeEndpoint, error) {
	var out []NodeEndpoint
	err := r.db.WithContext(ctx).
		Where("node_id = ? AND network_id = ?", nodeID, networkID).
		Order("updated_at desc, endpoint_id").
		Find(&out).Error
	return out, err
}

// GetNodeNatType reads the NAT observation from the newest endpoint row stored
// for the node.
func (r *PostgresRepository) GetNodeNatType(ctx context.Context, nodeID, networkID string) (string, error) {
	var record NodeEndpoint
	err := r.db.WithContext(ctx).
		Where("node_id = ? AND network_id = ?", nodeID, networkID).
		Order("updated_at desc, endpoint_id").
		First(&record).Error
	if err != nil {
		return "", err
	}
	return record.NatType, nil
}

// GetNodeConnectionState loads the latest stored connection state for a node
// pair in one network.
func (r *PostgresRepository) GetNodeConnectionState(ctx context.Context, networkID, nodeID, peerNodeID string) (NodeConnectionState, error) {
	var record NodeConnectionState
	err := r.db.WithContext(ctx).
		Where("network_id = ? AND node_id = ? AND peer_node_id = ?", networkID, nodeID, peerNodeID).
		First(&record).Error
	return record, err
}

// GetLatestDeviceConnectionState returns the newest connection-state row across
// every node owned by one device inside a network.
func (r *PostgresRepository) GetLatestDeviceConnectionState(ctx context.Context, networkID, deviceID string) (NodeConnectionState, error) {
	var record NodeConnectionState
	err := r.db.WithContext(ctx).
		Table("node_connection_states").
		Select("node_connection_states.*").
		Joins("join nodes on nodes.node_id = node_connection_states.node_id").
		Where("node_connection_states.network_id = ? AND nodes.device_id = ?", networkID, deviceID).
		Order("node_connection_states.updated_at desc, node_connection_states.state_id").
		First(&record).Error
	return record, err
}

// ListNodePathHealth returns recent path-health samples for a node pair ordered
// from newest to oldest.
func (r *PostgresRepository) ListNodePathHealth(ctx context.Context, networkID, nodeID, peerNodeID string) ([]NodePathHealth, error) {
	var out []NodePathHealth
	err := r.db.WithContext(ctx).
		Where("network_id = ? AND node_id = ? AND peer_node_id = ?", networkID, nodeID, peerNodeID).
		Order("sampled_at_ms desc, updated_at desc, health_id").
		Find(&out).Error
	return out, err
}

// ListRecentRelayNodePathHealth returns recent path-health rows that reference
// relay/DERP nodes so relay ranking can aggregate them.
func (r *PostgresRepository) ListRecentRelayNodePathHealth(ctx context.Context, cutoff int64) ([]NodePathHealth, error) {
	var out []NodePathHealth
	err := r.db.WithContext(ctx).
		Where("updated_at >= ? AND derp_node_id <> ''", cutoff).
		Order("sampled_at_ms desc, updated_at desc, health_id").
		Find(&out).Error
	return out, err
}

func (r *PostgresRepository) ListRecentNodePathHealth(ctx context.Context, cutoff int64) ([]NodePathHealth, error) {
	var out []NodePathHealth
	err := r.db.WithContext(ctx).
		Where("updated_at >= ?", cutoff).
		Order("updated_at desc, sampled_at_ms desc, health_id").
		Find(&out).Error
	return out, err
}

func (r *PostgresRepository) ListNodePathHealthSamplesByNetwork(ctx context.Context, networkID string, cutoffMs uint64) ([]NodePathHealthSample, error) {
	var out []NodePathHealthSample
	err := r.db.WithContext(ctx).
		Where("network_id = ? AND sampled_at_ms >= ?", networkID, cutoffMs).
		Order("sampled_at_ms desc, updated_at desc, sample_id").
		Find(&out).Error
	return out, err
}

func (r *PostgresRepository) ListRecentRelayNodeHeartbeats(ctx context.Context, cutoff int64) ([]RelayNodeHeartbeat, error) {
	var out []RelayNodeHeartbeat
	err := r.db.WithContext(ctx).
		Where("updated_at >= ?", cutoff).
		Order("updated_at desc, node_id").
		Find(&out).Error
	return out, err
}

func (r *PostgresRepository) ListRecentRelayPolicyExecutions(ctx context.Context, cutoff int64) ([]RelayPolicyExecution, error) {
	var out []RelayPolicyExecution
	err := r.db.WithContext(ctx).
		Where("updated_at >= ?", cutoff).
		Order("updated_at desc, network_id, device_id, policy_id").
		Find(&out).Error
	return out, err
}

func (r *PostgresRepository) GetRelayPolicyExecution(ctx context.Context, networkID, deviceID, policyID string) (RelayPolicyExecution, error) {
	var out RelayPolicyExecution
	err := r.db.WithContext(ctx).
		Where("network_id = ? AND device_id = ? AND policy_id = ?", networkID, deviceID, policyID).
		First(&out).Error
	return out, err
}

func (r *PostgresRepository) ListRelayPolicyTemplates(ctx context.Context, networkID string) ([]RelayPolicyTemplate, error) {
	var out []RelayPolicyTemplate
	err := r.db.WithContext(ctx).
		Where("network_id = '' OR network_id = ?", networkID).
		Order("network_id, name").
		Find(&out).Error
	return out, err
}

func (r *PostgresRepository) GetRelayPolicyTemplate(ctx context.Context, templateID string) (RelayPolicyTemplate, error) {
	var out RelayPolicyTemplate
	err := r.db.WithContext(ctx).Where("template_id = ?", templateID).First(&out).Error
	return out, err
}

// ListNodesByDevice returns all nodes registered under one device.
func (r *PostgresRepository) ListNodesByDevice(ctx context.Context, deviceID string) ([]Node, error) {
	var out []Node
	err := r.db.WithContext(ctx).Where("device_id = ?", deviceID).Order("node_id").Find(&out).Error
	return out, err
}

// ListNodes returns every node row ordered by node id.
func (r *PostgresRepository) ListNodes(ctx context.Context) ([]Node, error) {
	var out []Node
	err := r.db.WithContext(ctx).Order("node_id").Find(&out).Error
	return out, err
}
