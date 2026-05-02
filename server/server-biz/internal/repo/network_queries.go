package repo

import (
	"context"
	"strings"
	"time"

	"github.com/slan/server/server-biz/api/dto"
)

const assignmentRuntimeFreshnessWindow = 60 * time.Second

// GetAttachmentByID loads one subnet attachment by its stable public id.
func (r *PostgresRepository) GetAttachmentByID(ctx context.Context, attachmentID string) (dto.SubnetAttachment, error) {
	var model SubnetAttachment
	err := r.db.WithContext(ctx).Where("attachment_id = ?", attachmentID).First(&model).Error
	return model.ToDTO(), err
}

// GetNetworkByID loads one network row by its stable public id.
func (r *PostgresRepository) GetNetworkByID(ctx context.Context, networkID string) (Network, error) {
	var record Network
	err := r.db.WithContext(ctx).Where("network_id = ?", networkID).First(&record).Error
	return record, err
}

// GetOwnedNetworkByUser returns the one network currently owned by the user.
func (r *PostgresRepository) GetOwnedNetworkByUser(ctx context.Context, userID string) (Network, error) {
	var record Network
	err := r.db.WithContext(ctx).
		Where("owner_user_id = ?", userID).
		Order("network_id").
		First(&record).Error
	return record, err
}

func (r *PostgresRepository) ListNetworks(ctx context.Context) ([]Network, error) {
	var out []Network
	err := r.db.WithContext(ctx).Order("network_id").Find(&out).Error
	return out, err
}

// GetNetworkByJoinKey loads one network by its configured join key.
func (r *PostgresRepository) GetNetworkByJoinKey(ctx context.Context, joinKey string) (Network, error) {
	var record Network
	err := r.db.WithContext(ctx).
		Where("join_key = ?", strings.TrimSpace(joinKey)).
		First(&record).Error
	return record, err
}

// ListVisibleNetworksByUser returns networks owned by the user or visible
// through one of the user's devices being attached as a member.
func (r *PostgresRepository) ListVisibleNetworksByUser(ctx context.Context, userID string) ([]Network, error) {
	var user User
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&user).Error; err == nil {
		if strings.TrimSpace(user.ActiveNetworkID) != "" {
			var out []Network
			err := r.db.WithContext(ctx).
				Where("network_id = ?", user.ActiveNetworkID).
				Order("network_id").
				Find(&out).Error
			return out, err
		}
	}
	var out []Network
	err := r.db.WithContext(ctx).
		Table("networks").
		Select("distinct networks.*").
		Joins("left join network_members on network_members.network_id = networks.network_id").
		Joins("left join devices on devices.device_id = network_members.device_id").
		Where("networks.owner_user_id = ? OR (devices.user_id = ? AND network_members.status = ?)", userID, userID, "active").
		Order("networks.network_id").
		Find(&out).Error
	return out, err
}

// ListSubnetsByNetwork returns all declared subnets inside a network as DTOs.
func (r *PostgresRepository) ListSubnetsByNetwork(ctx context.Context, networkID string) ([]dto.Subnet, error) {
	var models []Subnet
	err := r.db.WithContext(ctx).
		Where("network_id = ?", networkID).
		Order("is_default desc, cidr, subnet_id").
		Find(&models).Error
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

// GetMemberByID loads one network membership by its stable public id.
func (r *PostgresRepository) GetMemberByID(ctx context.Context, memberID string) (dto.NetworkMember, error) {
	var model NetworkMember
	err := r.db.WithContext(ctx).
		Where("member_id = ?", memberID).
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

// ListMembersByDevice returns membership rows for one device across networks.
func (r *PostgresRepository) ListMembersByDevice(ctx context.Context, deviceID string) ([]dto.NetworkMember, error) {
	var models []NetworkMember
	err := r.db.WithContext(ctx).Where("device_id = ?", deviceID).Order("member_id").Find(&models).Error
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

func (r *PostgresRepository) ListActiveAttachmentsByUser(ctx context.Context, userID string) ([]dto.SubnetAttachment, error) {
	var models []SubnetAttachment
	err := r.db.WithContext(ctx).
		Table("subnet_attachments").
		Select("subnet_attachments.*").
		Joins("join devices on devices.device_id = subnet_attachments.device_id").
		Where("devices.user_id = ? AND subnet_attachments.status = ?", userID, "active").
		Order("subnet_attachments.attachment_id").
		Find(&models).Error
	if err != nil {
		return nil, err
	}
	out := make([]dto.SubnetAttachment, 0, len(models))
	for _, model := range models {
		out = append(out, model.ToDTO())
	}
	return out, nil
}

// ListAssignmentsByNetwork returns the flattened device-to-virtual-ip view for owner management pages.
func (r *PostgresRepository) ListAssignmentsByNetwork(ctx context.Context, networkID string) ([]dto.NetworkAssignment, error) {
	type row struct {
		AttachmentID            string
		NetworkID               string
		SubnetID                string
		DeviceID                string
		DeviceName              string
		DevicePlatform          string
		DeviceVersion           string
		UserID                  string
		UserEmail               string
		Role                    string
		Remark                  string
		VirtualIP               string
		Status                  string
		RuntimeControlReachable bool
		RuntimeNetworkOnline    bool
		RuntimeTunnelUp         bool
		RuntimeVirtualIP        string
		RuntimeLastSeenAt       int64
	}

	var rows []row
	err := r.db.WithContext(ctx).
		Table("subnet_attachments").
		Select(`
			subnet_attachments.attachment_id,
			subnet_attachments.network_id,
			subnet_attachments.subnet_id,
			subnet_attachments.device_id,
			devices.name AS device_name,
			devices.platform AS device_platform,
			devices.device_version AS device_version,
			devices.user_id,
			users.email AS user_email,
			network_members.role,
			subnet_attachments.remark,
			subnet_attachments.virtual_ip,
			subnet_attachments.status,
			COALESCE(device_network_states.control_reachable, false) AS runtime_control_reachable,
			COALESCE(device_network_states.network_online, false) AS runtime_network_online,
			COALESCE(device_network_states.tunnel_up, false) AS runtime_tunnel_up,
			COALESCE(device_network_states.virtual_ip, '') AS runtime_virtual_ip,
			COALESCE(device_network_states.last_seen_at, 0) AS runtime_last_seen_at
		`).
		Joins("join devices on devices.device_id = subnet_attachments.device_id").
		Joins("join users on users.user_id = devices.user_id").
		Joins("left join network_members on network_members.network_id = subnet_attachments.network_id and network_members.device_id = subnet_attachments.device_id").
		Joins("left join device_network_states on device_network_states.network_id = subnet_attachments.network_id and device_network_states.device_id = subnet_attachments.device_id").
		Where("subnet_attachments.network_id = ?", networkID).
		Order("subnet_attachments.attachment_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	freshCutoff := time.Now().Add(-assignmentRuntimeFreshnessWindow).Unix()
	out := make([]dto.NetworkAssignment, 0, len(rows))
	for _, row := range rows {
		out = append(out, dto.NetworkAssignment{
			AttachmentID:            row.AttachmentID,
			NetworkID:               row.NetworkID,
			SubnetID:                row.SubnetID,
			DeviceID:                row.DeviceID,
			DeviceName:              row.DeviceName,
			DevicePlatform:          row.DevicePlatform,
			DeviceVersion:           row.DeviceVersion,
			ConnectionType:          assignmentConnectionType(row.DevicePlatform),
			RuntimeControlReachable: row.RuntimeControlReachable,
			RuntimeNetworkOnline:    row.RuntimeNetworkOnline,
			RuntimeTunnelUp:         row.RuntimeTunnelUp,
			RuntimeVirtualIP:        row.RuntimeVirtualIP,
			RuntimeLastSeenAt:       row.RuntimeLastSeenAt,
			RuntimeStateFresh:       row.RuntimeLastSeenAt >= freshCutoff,
			UserID:                  row.UserID,
			UserEmail:               row.UserEmail,
			Role:                    row.Role,
			Remark:                  row.Remark,
			VirtualIP:               row.VirtualIP,
			Status:                  row.Status,
		})
	}
	return out, nil
}

func assignmentConnectionType(platform string) string {
	if strings.EqualFold(strings.TrimSpace(platform), "web") {
		return "console"
	}
	return "app"
}
