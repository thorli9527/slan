package repo

import (
	"context"

	"github.com/lib/pq"
	"github.com/slan/server/server-biz/api/dto"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Node struct {
	NodeID        string         `gorm:"column:node_id;primaryKey"`
	UserID        string         `gorm:"column:user_id;index;not null"`
	DeviceID      string         `gorm:"column:device_id;index;not null"`
	NodePublicKey string         `gorm:"column:node_public_key;not null"`
	Capabilities  pq.StringArray `gorm:"column:capabilities;type:text[];not null"`
}

func (Node) TableName() string { return "nodes" }

type NodeEndpoint struct {
	EndpointID string `gorm:"column:endpoint_id;primaryKey"`
	NodeID     string `gorm:"column:node_id;index;not null"`
	NetworkID  string `gorm:"column:network_id;index;not null"`
	NatType    string `gorm:"column:nat_type;not null;default:''"`
	Type       string `gorm:"column:type;not null"`
	Address    string `gorm:"column:address;not null"`
	UpdatedAt  int64  `gorm:"column:updated_at;not null"`
}

func (NodeEndpoint) TableName() string { return "node_endpoints" }

type NodeConnectionState struct {
	StateID    string `gorm:"column:state_id;primaryKey"`
	NetworkID  string `gorm:"column:network_id;index;not null;uniqueIndex:idx_node_peer_state"`
	NodeID     string `gorm:"column:node_id;index;not null;uniqueIndex:idx_node_peer_state"`
	PeerNodeID string `gorm:"column:peer_node_id;index;not null;uniqueIndex:idx_node_peer_state"`
	Path       string `gorm:"column:path;not null;default:''"`
	State      string `gorm:"column:state;not null"`
	Reason     string `gorm:"column:reason;not null;default:''"`
	UpdatedAt  int64  `gorm:"column:updated_at;not null"`
}

func (NodeConnectionState) TableName() string { return "node_connection_states" }

func (m Node) ToDTO(networkIDs []string) dto.Node {
	return dto.Node{
		NodeID:        m.NodeID,
		DeviceID:      m.DeviceID,
		NodePublicKey: m.NodePublicKey,
		NetworkIDs:    networkIDs,
		Capabilities:  append([]string(nil), m.Capabilities...),
	}
}

func (r *PostgresRepository) UpsertNode(ctx context.Context, record Node) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "node_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "device_id", "node_public_key", "capabilities"}),
	}).Create(&record).Error
}

func (r *PostgresRepository) GetNodeByID(ctx context.Context, nodeID string) (Node, error) {
	var record Node
	err := r.db.WithContext(ctx).Where("node_id = ?", nodeID).First(&record).Error
	return record, err
}

func (r *PostgresRepository) ListNodesByNetwork(ctx context.Context, networkID string) ([]Node, error) {
	var out []Node
	err := r.db.WithContext(ctx).
		Table("nodes").
		Select("distinct nodes.*").
		Joins("join network_members on network_members.device_id = nodes.device_id").
		Where("network_members.network_id = ?", networkID).
		Order("nodes.node_id").
		Find(&out).Error
	return out, err
}

func (m NodeEndpoint) ToDTO() dto.Endpoint {
	return dto.Endpoint{
		Type:      m.Type,
		Address:   m.Address,
		UpdatedAt: m.UpdatedAt,
	}
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

func (r *PostgresRepository) ListNodeEndpoints(ctx context.Context, nodeID, networkID string) ([]NodeEndpoint, error) {
	var out []NodeEndpoint
	err := r.db.WithContext(ctx).
		Where("node_id = ? AND network_id = ?", nodeID, networkID).
		Order("updated_at desc, endpoint_id").
		Find(&out).Error
	return out, err
}

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

func (r *PostgresRepository) UpsertNodeConnectionState(ctx context.Context, record NodeConnectionState) error {
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "network_id"},
			{Name: "node_id"},
			{Name: "peer_node_id"},
		},
		DoUpdates: clause.AssignmentColumns([]string{"path", "state", "reason", "updated_at"}),
	}).Create(&record).Error
}

func (r *PostgresRepository) GetNodeConnectionState(ctx context.Context, networkID, nodeID, peerNodeID string) (NodeConnectionState, error) {
	var record NodeConnectionState
	err := r.db.WithContext(ctx).
		Where("network_id = ? AND node_id = ? AND peer_node_id = ?", networkID, nodeID, peerNodeID).
		First(&record).Error
	return record, err
}

func (r *PostgresRepository) DeleteNodeEndpoints(ctx context.Context, nodeID, networkID string) error {
	return r.db.WithContext(ctx).
		Where("node_id = ? AND network_id = ?", nodeID, networkID).
		Delete(&NodeEndpoint{}).Error
}

func (r *PostgresRepository) DeleteNodeConnectionStates(ctx context.Context, nodeID, networkID string) error {
	return r.db.WithContext(ctx).
		Where("(node_id = ? OR peer_node_id = ?) AND network_id = ?", nodeID, nodeID, networkID).
		Delete(&NodeConnectionState{}).Error
}

func (r *PostgresRepository) ListNodesByDevice(ctx context.Context, deviceID string) ([]Node, error) {
	var out []Node
	err := r.db.WithContext(ctx).Where("device_id = ?", deviceID).Order("node_id").Find(&out).Error
	return out, err
}
