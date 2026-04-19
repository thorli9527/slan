package repo

import (
	"context"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
)

// GetNetworkByID loads one network row by its stable public id.
func (r *PostgresRepository) GetNetworkByID(ctx context.Context, networkID string) (Network, error) {
	var record Network
	err := r.db.WithContext(ctx).Where("network_id = ?", networkID).First(&record).Error
	return record, err
}

// ListVisibleNetworksByUser returns networks owned by the user or visible
// through one of the user's devices being attached as a member.
func (r *PostgresRepository) ListVisibleNetworksByUser(ctx context.Context, userID string) ([]Network, error) {
	var out []Network
	err := r.db.WithContext(ctx).
		Table("networks").
		Select("distinct networks.*").
		Joins("left join network_members on network_members.network_id = networks.network_id").
		Joins("left join devices on devices.device_id = network_members.device_id").
		Where("networks.owner_user_id = ? OR devices.user_id = ?", userID, userID).
		Order("networks.network_id").
		Find(&out).Error
	return out, err
}

// ListSubnetsByNetwork returns all declared subnets inside a network as DTOs.
func (r *PostgresRepository) ListSubnetsByNetwork(ctx context.Context, networkID string) ([]dto.Subnet, error) {
	var models []Subnet
	err := r.db.WithContext(ctx).Where("network_id = ?", networkID).Order("subnet_id").Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]dto.Subnet, 0, len(models))
	for _, model := range models {
		out = append(out, model.ToDTO())
	}
	return out, nil
}

// GetSubnetByID loads a single subnet and converts it to a DTO.
func (r *PostgresRepository) GetSubnetByID(ctx context.Context, subnetID string) (dto.Subnet, error) {
	var model Subnet
	err := r.db.WithContext(ctx).Where("subnet_id = ?", subnetID).First(&model).Error
	return model.ToDTO(), err
}

// FindSubnetByName performs a case-insensitive subnet lookup scoped to one
// network.
func (r *PostgresRepository) FindSubnetByName(ctx context.Context, networkID, name string) (dto.Subnet, error) {
	var model Subnet
	err := r.db.WithContext(ctx).
		Where("network_id = ? AND lower(name) = lower(?)", networkID, strings.TrimSpace(name)).
		First(&model).Error
	return model.ToDTO(), err
}

// GetMemberByNetworkDevice loads the membership row connecting one device to a
// network.
func (r *PostgresRepository) GetMemberByNetworkDevice(ctx context.Context, networkID, deviceID string) (dto.NetworkMember, error) {
	var model NetworkMember
	err := r.db.WithContext(ctx).
		Where("network_id = ? AND device_id = ?", networkID, deviceID).
		First(&model).Error
	return model.ToDTO(), err
}

// ListMembersByNetwork returns every member device currently attached to the
// network.
func (r *PostgresRepository) ListMembersByNetwork(ctx context.Context, networkID string) ([]dto.NetworkMember, error) {
	var models []NetworkMember
	err := r.db.WithContext(ctx).Where("network_id = ?", networkID).Order("member_id").Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]dto.NetworkMember, 0, len(models))
	for _, model := range models {
		out = append(out, model.ToDTO())
	}
	return out, nil
}

// GetAttachmentBySubnetDevice loads the subnet attachment row for one device.
func (r *PostgresRepository) GetAttachmentBySubnetDevice(ctx context.Context, subnetID, deviceID string) (dto.SubnetAttachment, error) {
	var model SubnetAttachment
	err := r.db.WithContext(ctx).
		Where("subnet_id = ? AND device_id = ?", subnetID, deviceID).
		First(&model).Error
	return model.ToDTO(), err
}

// ListAttachmentsByDevice returns every subnet attachment currently owned by
// the device.
func (r *PostgresRepository) ListAttachmentsByDevice(ctx context.Context, deviceID string) ([]dto.SubnetAttachment, error) {
	var models []SubnetAttachment
	err := r.db.WithContext(ctx).Where("device_id = ?", deviceID).Order("attachment_id").Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]dto.SubnetAttachment, 0, len(models))
	for _, model := range models {
		out = append(out, model.ToDTO())
	}
	return out, nil
}

// ListAttachmentsBySubnet returns every device attached to the subnet.
func (r *PostgresRepository) ListAttachmentsBySubnet(ctx context.Context, subnetID string) ([]dto.SubnetAttachment, error) {
	var models []SubnetAttachment
	err := r.db.WithContext(ctx).Where("subnet_id = ?", subnetID).Order("attachment_id").Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]dto.SubnetAttachment, 0, len(models))
	for _, model := range models {
		out = append(out, model.ToDTO())
	}
	return out, nil
}
