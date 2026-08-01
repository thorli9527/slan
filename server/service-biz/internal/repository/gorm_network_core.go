package repository

import (
	"context"
	"strings"

	"github.com/slan/service-biz/internal/model"
	"gorm.io/gorm"
)

func (s *GormStore) ListNetworksByOwner(ctx context.Context, ownerID string) ([]model.Network, error) {
	return listModels(s.db.WithContext(ctx).Where("owner_id = ?", ownerID).Order("network_id asc"), func(row gormNetworkRecord) model.Network {
		return row.model()
	})
}

func (s *GormStore) GetNetwork(ctx context.Context, networkID string) (model.Network, bool, error) {
	return firstModel(s.db.WithContext(ctx).Where("network_id = ?", networkID), func(row gormNetworkRecord) model.Network {
		return row.model()
	})
}

func (s *GormStore) GetNetworkVersion(ctx context.Context, networkID string) (model.NetworkConfigVersion, bool, error) {
	return firstModel(s.db.WithContext(ctx).Where("network_id = ?", networkID), func(row gormNetworkConfigVersionRecord) model.NetworkConfigVersion {
		return row.model()
	})
}

func (s *GormStore) ListNetworksByDevice(ctx context.Context, deviceID string) ([]model.Network, error) {
	memberships, err := listModels(
		s.db.WithContext(ctx).Where("device_id = ? AND enabled = ? AND member_status = ?", deviceID, true, "active").Order("network_id asc"),
		func(row gormNetworkDeviceRecord) model.NetworkDevice { return row.model() },
	)
	if err != nil {
		return nil, err
	}
	if len(memberships) == 0 {
		return []model.Network{}, nil
	}
	networks := make([]model.Network, 0, len(memberships))
	seen := make(map[string]struct{}, len(memberships))
	for _, membership := range memberships {
		if _, ok := seen[membership.NetworkID]; ok {
			continue
		}
		network, ok, err := s.GetNetwork(ctx, membership.NetworkID)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		seen[membership.NetworkID] = struct{}{}
		networks = append(networks, network)
	}
	return networks, nil
}

func (s *GormStore) SaveNetwork(ctx context.Context, network model.Network) error {
	row := networkRecordFromModel(network)
	return upsertByColumns(s.db.WithContext(ctx), &row, []string{"network_id"}, []string{"owner_id", "name", "cidr", "intra_group_policy", "default", "status", "created_at", "updated_at"})
}

func (s *GormStore) SaveNetworkVersion(ctx context.Context, item model.NetworkConfigVersion) error {
	row := networkConfigVersionRecordFromModel(item)
	return upsertByColumns(s.db.WithContext(ctx), &row, []string{"network_id"}, []string{"version", "reason", "created_at", "updated_at"})
}

func (s *GormStore) DeleteNetwork(ctx context.Context, networkID string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		securityGroupIDs := tx.Model(&gormSecurityGroupRecord{}).
			Select("security_group_id").
			Where("network_id = ?", networkID)

		steps := []func() error{
			func() error {
				return tx.Delete(&gormSecurityRuleRecord{}, "security_group_id IN (?)", securityGroupIDs).Error
			},
			func() error { return tx.Delete(&gormSecurityGroupRecord{}, "network_id = ?", networkID).Error },
			func() error { return tx.Delete(&gormPublicMappingRecord{}, "network_id = ?", networkID).Error },
			func() error { return tx.Delete(&gormDNSRecordRecord{}, "network_id = ?", networkID).Error },
			func() error { return tx.Delete(&gormDNSZoneRecord{}, "network_id = ?", networkID).Error },
			func() error { return tx.Delete(&gormNetworkDeviceRecord{}, "network_id = ?", networkID).Error },
			func() error {
				return tx.Delete(&gormNetworkDeviceGroupReferenceRecord{}, "network_id = ?", networkID).Error
			},
			func() error { return tx.Delete(&gormDeviceInviteRecord{}, "network_id = ?", networkID).Error },
			func() error { return tx.Delete(&gormBootstrapKeyRecord{}, "network_id = ?", networkID).Error },
			func() error { return tx.Delete(&gormNetworkRecord{}, "network_id = ?", networkID).Error },
		}
		for _, step := range steps {
			if err := step(); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *GormStore) ListNetworkDevices(ctx context.Context, networkID string) ([]model.NetworkDevice, error) {
	return listModels(s.db.WithContext(ctx).Where("network_id = ?", networkID).Order("device_id asc"), func(row gormNetworkDeviceRecord) model.NetworkDevice {
		return row.model()
	})
}

func (s *GormStore) GetNetworkDevice(ctx context.Context, networkID, deviceID string) (model.NetworkDevice, bool, error) {
	return firstModel(s.db.WithContext(ctx).Where("network_id = ? AND device_id = ?", networkID, deviceID), func(row gormNetworkDeviceRecord) model.NetworkDevice {
		return row.model()
	})
}

func (s *GormStore) SaveNetworkDevice(ctx context.Context, item model.NetworkDevice) error {
	row := networkDeviceRecordFromModel(item)
	return upsertByColumns(s.db.WithContext(ctx), &row, []string{"network_id", "device_id"}, []string{"enabled", "member_status", "membership_source", "presence_status", "mqtt_connected", "virtual_ip", "last_seen_at", "last_heartbeat_at", "last_runtime_state_at", "last_endpoint_at", "last_path_health_at", "endpoints", "nat_type", "active_path", "path_observed_at", "relay_transport", "relay_endpoint", "derp_node_id", "peer_node_id", "path_score", "observed_rtt_ms", "packet_loss_ppm", "relay_mtu", "max_frame_payload", "ticket_expires_at", "ticket_renew_due", "path_downgrades", "path_upgrades", "last_path_change", "created_at", "updated_at"})
}

func (s *GormStore) DeleteNetworkDevice(ctx context.Context, networkID, deviceID string) error {
	return s.db.WithContext(ctx).Delete(&gormNetworkDeviceRecord{}, "network_id = ? AND device_id = ?", networkID, deviceID).Error
}

func (s *GormStore) ListNetworkDeviceGroupReferences(_ context.Context, networkID string) ([]model.NetworkDeviceGroupReference, error) {
	return listModels(
		s.db.Where("network_id = ?", strings.TrimSpace(networkID)).Order("group_id asc"),
		func(row gormNetworkDeviceGroupReferenceRecord) model.NetworkDeviceGroupReference {
			return model.NetworkDeviceGroupReference{
				NetworkID: row.NetworkID,
				GroupID:   row.GroupID,
				CreatedAt: row.CreatedAt,
				UpdatedAt: row.UpdatedAt,
			}
		},
	)
}

func (s *GormStore) SaveNetworkDeviceGroupReference(_ context.Context, item model.NetworkDeviceGroupReference) error {
	row := gormNetworkDeviceGroupReferenceRecord{
		NetworkID: strings.TrimSpace(item.NetworkID),
		GroupID:   strings.TrimSpace(item.GroupID),
		CreatedAt: item.CreatedAt,
		UpdatedAt: item.UpdatedAt,
	}
	return upsertByColumns(s.db, &row, []string{"network_id", "group_id"}, []string{"updated_at"})
}

func (s *GormStore) DeleteNetworkDeviceGroupReference(_ context.Context, networkID, groupID string) error {
	return s.db.Delete(
		&gormNetworkDeviceGroupReferenceRecord{},
		"network_id = ? AND group_id = ?",
		strings.TrimSpace(networkID),
		strings.TrimSpace(groupID),
	).Error
}

func (s *GormStore) DeleteNetworkDeviceGroupReferencesByGroup(_ context.Context, groupID string) error {
	return s.db.Delete(&gormNetworkDeviceGroupReferenceRecord{}, "group_id = ?", strings.TrimSpace(groupID)).Error
}

func (s *GormStore) ListDeviceInvitesByUser(_ context.Context, userID string) ([]model.DeviceInvite, error) {
	userID = strings.TrimSpace(userID)
	return listModels(s.db.Where("user_id = ? OR inviter_user_id = ?", userID, userID).Order("invite_id asc"), func(row gormDeviceInviteRecord) model.DeviceInvite {
		return row.model()
	})
}

func (s *GormStore) ListDeviceInvitesByNetwork(_ context.Context, networkID string) ([]model.DeviceInvite, error) {
	return listModels(s.db.Where("network_id = ?", networkID).Order("invite_id asc"), func(row gormDeviceInviteRecord) model.DeviceInvite {
		return row.model()
	})
}

func (s *GormStore) GetDeviceInvite(_ context.Context, inviteID string) (model.DeviceInvite, bool, error) {
	return firstModel(s.db.Where("invite_id = ?", inviteID), func(row gormDeviceInviteRecord) model.DeviceInvite {
		return row.model()
	})
}

func (s *GormStore) GetDeviceInviteByCode(_ context.Context, inviteCode string) (model.DeviceInvite, bool, error) {
	return firstModel(s.db.Where("invite_code = ?", strings.TrimSpace(inviteCode)), func(row gormDeviceInviteRecord) model.DeviceInvite {
		return row.model()
	})
}

func (s *GormStore) SaveDeviceInvite(_ context.Context, invite model.DeviceInvite) error {
	row := deviceInviteRecordFromModel(invite)
	row.InviteCode = strings.TrimSpace(row.InviteCode)
	return upsertByColumns(s.db, &row, []string{"invite_id"}, []string{"invite_code", "inviter_user_id", "network_id", "device_id", "user_id", "status", "created_at", "expires_at", "accepted_at"})
}
