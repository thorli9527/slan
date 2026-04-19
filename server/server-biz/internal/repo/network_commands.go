package repo

import (
	"context"

	"github.com/slan/server/server-biz/api/dto"
	"gorm.io/gorm"
)

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
