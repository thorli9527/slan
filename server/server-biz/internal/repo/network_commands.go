package repo

import (
	"context"

	"github.com/slan/server/server-biz/api/dto"
	"gorm.io/gorm"
)

func (r *PostgresRepository) CreateNetworkWithDefaultSubnet(ctx context.Context, ownerUserID string, network dto.Network, subnet dto.Subnet) error {
	return r.CreateNetworkWithSubnets(ctx, ownerUserID, network, []dto.Subnet{subnet})
}

func (r *PostgresRepository) CreateNetworkWithSubnets(ctx context.Context, ownerUserID string, network dto.Network, subnets []dto.Subnet) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		networkModel := Network{
			NetworkID:         network.NetworkID,
			OwnerUserID:       ownerUserID,
			Name:              network.Name,
			Description:       network.Description,
			DefaultSubnetID:   network.DefaultSubnetID,
			DefaultSubnetCIDR: network.DefaultSubnetCIDR,
			DNSServers:        "",
			DNSSearchDomains:  "",
			DNSWildcards:      "",
			JoinKey:           "",
		}
		if err := tx.Create(&networkModel).Error; err != nil {
			return err
		}
		for _, subnet := range subnets {
			subnetModel := Subnet{
				SubnetID:          subnet.SubnetID,
				NetworkID:         subnet.NetworkID,
				Name:              subnet.Name,
				CIDR:              subnet.CIDR,
				Remark:            subnet.Remark,
				GatewayIP:         subnet.GatewayIP,
				AllocationStartIP: subnet.AllocationStartIP,
				AllocationEndIP:   subnet.AllocationEndIP,
				IsDefault:         subnet.IsDefault,
				Status:            subnet.Status,
			}
			if err := tx.Create(&subnetModel).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *PostgresRepository) CreateSubnet(ctx context.Context, subnet dto.Subnet) error {
	model := Subnet{
		SubnetID:          subnet.SubnetID,
		NetworkID:         subnet.NetworkID,
		Name:              subnet.Name,
		CIDR:              subnet.CIDR,
		Remark:            subnet.Remark,
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
		CreatedAt: member.CreatedAt,
		Status:    member.Status,
	}
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *PostgresRepository) UpdateMemberStatus(ctx context.Context, memberID, status string) error {
	return r.db.WithContext(ctx).
		Model(&NetworkMember{}).
		Where("member_id = ?", memberID).
		Update("status", status).Error
}

func (r *PostgresRepository) DeleteMemberByID(ctx context.Context, memberID string) error {
	return r.db.WithContext(ctx).
		Where("member_id = ?", memberID).
		Delete(&NetworkMember{}).Error
}

func (r *PostgresRepository) CreateAttachment(ctx context.Context, attachment dto.SubnetAttachment) error {
	model := SubnetAttachment{
		AttachmentID: attachment.AttachmentID,
		NetworkID:    attachment.NetworkID,
		SubnetID:     attachment.SubnetID,
		DeviceID:     attachment.DeviceID,
		VirtualIP:    attachment.VirtualIP,
		Remark:       attachment.Remark,
		Status:       attachment.Status,
	}
	return r.db.WithContext(ctx).Create(&model).Error
}

func (r *PostgresRepository) DeleteMembersByDeviceExceptNetwork(ctx context.Context, deviceID, keepNetworkID string) error {
	query := r.db.WithContext(ctx).Where("device_id = ?", deviceID)
	if keepNetworkID != "" {
		query = query.Where("network_id <> ?", keepNetworkID)
	}
	return query.Delete(&NetworkMember{}).Error
}

func (r *PostgresRepository) DeleteAttachmentsByDeviceExceptNetwork(ctx context.Context, deviceID, keepNetworkID string) error {
	query := r.db.WithContext(ctx).Where("device_id = ?", deviceID)
	if keepNetworkID != "" {
		query = query.Where("network_id <> ?", keepNetworkID)
	}
	return query.Delete(&SubnetAttachment{}).Error
}

func (r *PostgresRepository) DeleteAttachmentsByDeviceInNetwork(ctx context.Context, deviceID, networkID string) error {
	return r.db.WithContext(ctx).
		Where("device_id = ? AND network_id = ?", deviceID, networkID).
		Delete(&SubnetAttachment{}).Error
}

func (r *PostgresRepository) ClearAttachmentVirtualIP(ctx context.Context, attachmentID string) error {
	return r.db.WithContext(ctx).
		Model(&SubnetAttachment{}).
		Where("attachment_id = ?", attachmentID).
		Update("virtual_ip", "").Error
}

func (r *PostgresRepository) UpdateNetworkMetadata(ctx context.Context, networkID, name, description, defaultSubnetCIDR string) error {
	return r.db.WithContext(ctx).
		Model(&Network{}).
		Where("network_id = ?", networkID).
		Updates(map[string]any{
			"name":                name,
			"description":         description,
			"default_subnet_cidr": defaultSubnetCIDR,
		}).Error
}

func (r *PostgresRepository) UpdateNetworkDNS(ctx context.Context, networkID string, dns dto.DNSConfig) error {
	return r.db.WithContext(ctx).
		Model(&Network{}).
		Where("network_id = ?", networkID).
		Updates(map[string]any{
			"dns_servers":        encodeCSVList(dns.Servers),
			"dns_search_domains": encodeCSVList(dns.SearchDomains),
			"dns_wildcards":      encodeCSVList(dns.Wildcards),
		}).Error
}

func (r *PostgresRepository) UpdateNetworkJoinKey(ctx context.Context, networkID, joinKey string) error {
	return r.db.WithContext(ctx).
		Model(&Network{}).
		Where("network_id = ?", networkID).
		Update("join_key", joinKey).Error
}

func (r *PostgresRepository) ConsumeNetworkByJoinKey(ctx context.Context, joinKey string) (Network, error) {
	var record Network
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.
			Where("join_key = ?", joinKey).
			First(&record).Error; err != nil {
			return err
		}
		return tx.
			Model(&Network{}).
			Where("network_id = ? AND join_key = ?", record.NetworkID, joinKey).
			Update("join_key", "").Error
	})
	return record, err
}

func (r *PostgresRepository) UpdateSubnetRange(ctx context.Context, subnet dto.Subnet) error {
	return r.db.WithContext(ctx).
		Model(&Subnet{}).
		Where("subnet_id = ?", subnet.SubnetID).
		Updates(map[string]any{
			"cidr":                subnet.CIDR,
			"remark":              subnet.Remark,
			"gateway_ip":          subnet.GatewayIP,
			"allocation_start_ip": subnet.AllocationStartIP,
			"allocation_end_ip":   subnet.AllocationEndIP,
			"is_default":          subnet.IsDefault,
			"status":              subnet.Status,
		}).Error
}

func (r *PostgresRepository) UpdateAttachmentVirtualIP(ctx context.Context, attachmentID, virtualIP string) error {
	return r.db.WithContext(ctx).
		Model(&SubnetAttachment{}).
		Where("attachment_id = ?", attachmentID).
		Update("virtual_ip", virtualIP).Error
}

func (r *PostgresRepository) SuspendAttachment(ctx context.Context, attachmentID string) error {
	return r.db.WithContext(ctx).
		Model(&SubnetAttachment{}).
		Where("attachment_id = ?", attachmentID).
		Updates(map[string]any{
			"status":     "suspended",
			"virtual_ip": "",
		}).Error
}

func (r *PostgresRepository) UpdateAttachmentRemark(ctx context.Context, attachmentID, remark string) error {
	return r.db.WithContext(ctx).
		Model(&SubnetAttachment{}).
		Where("attachment_id = ?", attachmentID).
		Update("remark", remark).Error
}
