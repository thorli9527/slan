package repo

import (
	"context"
	"strings"

	"github.com/slan/server/server-biz/api/dto"
	"gorm.io/gorm"
)

type Network struct {
	NetworkID         string `gorm:"column:network_id;primaryKey"`
	OwnerUserID       string `gorm:"column:owner_user_id;index;not null"`
	Name              string `gorm:"column:name;not null"`
	Description       string `gorm:"column:description;not null;default:''"`
	DefaultSubnetID   string `gorm:"column:default_subnet_id;not null"`
	DefaultSubnetCIDR string `gorm:"column:default_subnet_cidr;not null"`
}

func (Network) TableName() string { return "networks" }

func (m Network) ToDTO() dto.Network {
	return dto.Network{
		NetworkID:         m.NetworkID,
		Name:              m.Name,
		Description:       m.Description,
		DefaultSubnetID:   m.DefaultSubnetID,
		DefaultSubnetCIDR: m.DefaultSubnetCIDR,
	}
}

type Subnet struct {
	SubnetID          string `gorm:"column:subnet_id;primaryKey"`
	NetworkID         string `gorm:"column:network_id;index;not null"`
	Name              string `gorm:"column:name;not null"`
	CIDR              string `gorm:"column:cidr;not null"`
	GatewayIP         string `gorm:"column:gateway_ip;not null;default:''"`
	AllocationStartIP string `gorm:"column:allocation_start_ip;not null;default:''"`
	AllocationEndIP   string `gorm:"column:allocation_end_ip;not null;default:''"`
	IsDefault         bool   `gorm:"column:is_default;not null;default:false"`
	Status            string `gorm:"column:status;not null;default:'active'"`
}

func (Subnet) TableName() string { return "subnets" }

func (m Subnet) ToDTO() dto.Subnet {
	return dto.Subnet{
		SubnetID:          m.SubnetID,
		NetworkID:         m.NetworkID,
		Name:              m.Name,
		CIDR:              m.CIDR,
		GatewayIP:         m.GatewayIP,
		AllocationStartIP: m.AllocationStartIP,
		AllocationEndIP:   m.AllocationEndIP,
		IsDefault:         m.IsDefault,
		Status:            m.Status,
	}
}

type NetworkMember struct {
	MemberID  string `gorm:"column:member_id;primaryKey"`
	NetworkID string `gorm:"column:network_id;index;not null;uniqueIndex:idx_network_device"`
	DeviceID  string `gorm:"column:device_id;index;not null;uniqueIndex:idx_network_device"`
	Role      string `gorm:"column:role;not null"`
	Status    string `gorm:"column:status;not null;default:'active'"`
}

func (NetworkMember) TableName() string { return "network_members" }

func (m NetworkMember) ToDTO() dto.NetworkMember {
	return dto.NetworkMember{
		MemberID:  m.MemberID,
		NetworkID: m.NetworkID,
		DeviceID:  m.DeviceID,
		Role:      m.Role,
		Status:    m.Status,
	}
}

type SubnetAttachment struct {
	AttachmentID string `gorm:"column:attachment_id;primaryKey"`
	NetworkID    string `gorm:"column:network_id;index;not null"`
	SubnetID     string `gorm:"column:subnet_id;index;not null;uniqueIndex:idx_subnet_device"`
	DeviceID     string `gorm:"column:device_id;index;not null;uniqueIndex:idx_subnet_device"`
	VirtualIP    string `gorm:"column:virtual_ip;not null;default:'';uniqueIndex:idx_subnet_ip"`
	Status       string `gorm:"column:status;not null;default:'active'"`
}

func (SubnetAttachment) TableName() string { return "subnet_attachments" }

func (m SubnetAttachment) ToDTO() dto.SubnetAttachment {
	return dto.SubnetAttachment{
		AttachmentID: m.AttachmentID,
		NetworkID:    m.NetworkID,
		SubnetID:     m.SubnetID,
		DeviceID:     m.DeviceID,
		VirtualIP:    m.VirtualIP,
		Status:       m.Status,
	}
}

func (r *PostgresRepository) CreateNetworkWithDefaultSubnet(ctx context.Context, ownerUserID string, network dto.Network, subnet dto.Subnet) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		networkModel := Network{
			NetworkID:         network.NetworkID,
			OwnerUserID:       ownerUserID,
			Name:              network.Name,
			Description:       network.Description,
			DefaultSubnetID:   network.DefaultSubnetID,
			DefaultSubnetCIDR: network.DefaultSubnetCIDR,
		}
		subnetModel := Subnet{
			SubnetID:          subnet.SubnetID,
			NetworkID:         subnet.NetworkID,
			Name:              subnet.Name,
			CIDR:              subnet.CIDR,
			GatewayIP:         subnet.GatewayIP,
			AllocationStartIP: subnet.AllocationStartIP,
			AllocationEndIP:   subnet.AllocationEndIP,
			IsDefault:         subnet.IsDefault,
			Status:            subnet.Status,
		}
		if err := tx.Create(&networkModel).Error; err != nil {
			return err
		}
		return tx.Create(&subnetModel).Error
	})
}

func (r *PostgresRepository) GetNetworkByID(ctx context.Context, networkID string) (Network, error) {
	var record Network
	err := r.db.WithContext(ctx).Where("network_id = ?", networkID).First(&record).Error
	return record, err
}

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

func (r *PostgresRepository) CreateSubnet(ctx context.Context, subnet dto.Subnet) error {
	model := Subnet{
		SubnetID:          subnet.SubnetID,
		NetworkID:         subnet.NetworkID,
		Name:              subnet.Name,
		CIDR:              subnet.CIDR,
		GatewayIP:         subnet.GatewayIP,
		AllocationStartIP: subnet.AllocationStartIP,
		AllocationEndIP:   subnet.AllocationEndIP,
		IsDefault:         subnet.IsDefault,
		Status:            subnet.Status,
	}
	return r.db.WithContext(ctx).Create(&model).Error
}

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

func (r *PostgresRepository) GetSubnetByID(ctx context.Context, subnetID string) (dto.Subnet, error) {
	var model Subnet
	err := r.db.WithContext(ctx).Where("subnet_id = ?", subnetID).First(&model).Error
	return model.ToDTO(), err
}

func (r *PostgresRepository) FindSubnetByName(ctx context.Context, networkID, name string) (dto.Subnet, error) {
	var model Subnet
	err := r.db.WithContext(ctx).
		Where("network_id = ? AND lower(name) = lower(?)", networkID, strings.TrimSpace(name)).
		First(&model).Error
	return model.ToDTO(), err
}

func (r *PostgresRepository) GetMemberByNetworkDevice(ctx context.Context, networkID, deviceID string) (dto.NetworkMember, error) {
	var model NetworkMember
	err := r.db.WithContext(ctx).
		Where("network_id = ? AND device_id = ?", networkID, deviceID).
		First(&model).Error
	return model.ToDTO(), err
}

func (r *PostgresRepository) CreateMember(ctx context.Context, member dto.NetworkMember) error {
	model := NetworkMember{
		MemberID:  member.MemberID,
		NetworkID: member.NetworkID,
		DeviceID:  member.DeviceID,
		Role:      member.Role,
		Status:    member.Status,
	}
	return r.db.WithContext(ctx).Create(&model).Error
}

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

func (r *PostgresRepository) GetAttachmentBySubnetDevice(ctx context.Context, subnetID, deviceID string) (dto.SubnetAttachment, error) {
	var model SubnetAttachment
	err := r.db.WithContext(ctx).
		Where("subnet_id = ? AND device_id = ?", subnetID, deviceID).
		First(&model).Error
	return model.ToDTO(), err
}

func (r *PostgresRepository) CreateAttachment(ctx context.Context, attachment dto.SubnetAttachment) error {
	model := SubnetAttachment{
		AttachmentID: attachment.AttachmentID,
		NetworkID:    attachment.NetworkID,
		SubnetID:     attachment.SubnetID,
		DeviceID:     attachment.DeviceID,
		VirtualIP:    attachment.VirtualIP,
		Status:       attachment.Status,
	}
	return r.db.WithContext(ctx).Create(&model).Error
}

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
