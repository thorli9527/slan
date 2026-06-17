package biz

import (
	"context"
	"database/sql"
	"strings"
)

func (s *Store) persistPostgresNetworkDeviceUpsertTxLocked(ctx context.Context, tx *sql.Tx, membership NetworkDevice) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `insert into network_devices(id,network_id,device_id,owner_user_id,alias,enabled,status,created_at,updated_at)
		values($1,$2,$3,$4,$5,$6,$7,to_timestamp($8),to_timestamp($9))
		on conflict(network_id, device_id) do update set
			owner_user_id=excluded.owner_user_id,
			alias=excluded.alias,
			enabled=excluded.enabled,
			status=excluded.status,
			updated_at=excluded.updated_at`,
		membership.NetworkDeviceID, membership.NetworkID, membership.DeviceID, membership.OwnerUserID, membership.Alias, membership.Enabled, membership.Status, membership.CreatedAt, membership.UpdatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresNetworkDeviceDeleteTxLocked(ctx context.Context, tx *sql.Tx, networkID, deviceID string) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `delete from network_devices where network_id=$1 and device_id=$2`, networkID, deviceID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresDNSZoneUpsertTxLocked(ctx context.Context, tx *sql.Tx, zone NetworkDNSZone) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `insert into network_dns_zones(id,network_id,zone_name,expose_global,status,created_at)
		values($1,$2,$3,$4,$5,to_timestamp($6))
		on conflict(id) do update set
			zone_name=excluded.zone_name,
			expose_global=excluded.expose_global,
			status=excluded.status`,
		zone.ZoneID, zone.NetworkID, zone.ZoneName, zone.ExposeGlobal, zone.Status, zone.CreatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresDNSZoneDeleteTxLocked(ctx context.Context, tx *sql.Tx, networkID, zoneID string) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `delete from network_dns_records where network_id=$1 and zone_id=$2`, networkID, zoneID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `delete from network_dns_zones where network_id=$1 and id=$2`, networkID, zoneID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresDNSRecordUpsertTxLocked(ctx context.Context, tx *sql.Tx, record NetworkDNSRecord) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	var targetDeviceID any
	if strings.TrimSpace(record.TargetDeviceID) != "" {
		targetDeviceID = record.TargetDeviceID
	}
	var targetIP any
	if strings.TrimSpace(record.TargetIP) != "" {
		targetIP = record.TargetIP
	}
	if _, err := tx.ExecContext(ctx, `insert into network_dns_records(id,zone_id,network_id,name,fqdn,record_type,target_device_id,target_ip,cname,port,ttl,status,created_at)
		values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,to_timestamp($13))
		on conflict(id) do update set
			name=excluded.name,
			fqdn=excluded.fqdn,
			record_type=excluded.record_type,
			target_device_id=excluded.target_device_id,
			target_ip=excluded.target_ip,
			cname=excluded.cname,
			port=excluded.port,
			ttl=excluded.ttl,
			status=excluded.status`,
		record.RecordID, record.ZoneID, record.NetworkID, record.Name, record.FQDN, record.RecordType, targetDeviceID, targetIP, record.CNAME, record.Port, record.TTL, record.Status, record.CreatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresDNSRecordDeleteTxLocked(ctx context.Context, tx *sql.Tx, networkID, recordID string) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `delete from network_dns_records where network_id=$1 and id=$2`, networkID, recordID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresPublicMappingUpsertTxLocked(ctx context.Context, tx *sql.Tx, mapping PublicDomainMapping) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `insert into public_domain_mappings(id,network_id,alias,public_domain,source_record,device_id,protocol,port,external_port,status,created_at,updated_at)
		values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,to_timestamp($11),to_timestamp($12))
		on conflict(id) do update set
			alias=excluded.alias,
			public_domain=excluded.public_domain,
			source_record=excluded.source_record,
			device_id=excluded.device_id,
			protocol=excluded.protocol,
			port=excluded.port,
			external_port=excluded.external_port,
			status=excluded.status,
			updated_at=excluded.updated_at`,
		mapping.MappingID, mapping.NetworkID, mapping.Alias, mapping.PublicDomain, mapping.SourceRecord, mapping.DeviceID, mapping.Protocol, mapping.Port, mapping.ExternalPort, mapping.Status, mapping.CreatedAt, mapping.UpdatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresPublicMappingDeleteTxLocked(ctx context.Context, tx *sql.Tx, networkID, mappingID string) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `delete from public_domain_mappings where network_id=$1 and id=$2`, networkID, mappingID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresSecurityGroupUpsertTxLocked(ctx context.Context, tx *sql.Tx, group SecurityGroup) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `insert into security_groups(id,network_id,name,description,status,created_at)
		values($1,$2,$3,$4,$5,to_timestamp($6))
		on conflict(id) do update set
			name=excluded.name,
			description=excluded.description,
			status=excluded.status`,
		group.SecurityGroupID, group.NetworkID, group.Name, group.Description, group.Status, group.CreatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresSecurityGroupDeleteTxLocked(ctx context.Context, tx *sql.Tx, networkID, securityGroupID string) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `delete from security_group_rules where security_group_id=$1`, securityGroupID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `delete from security_groups where network_id=$1 and id=$2`, networkID, securityGroupID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresSecurityGroupRuleUpsertTxLocked(ctx context.Context, tx *sql.Tx, rule SecurityGroupRule) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `insert into security_group_rules(id,security_group_id,direction,priority,action,protocol,port_from,port_to,peer_type,peer_value,description,enabled,created_at)
		values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,to_timestamp($13))
		on conflict(id) do update set
			direction=excluded.direction,
			priority=excluded.priority,
			action=excluded.action,
			protocol=excluded.protocol,
			port_from=excluded.port_from,
			port_to=excluded.port_to,
			peer_type=excluded.peer_type,
			peer_value=excluded.peer_value,
			description=excluded.description,
			enabled=excluded.enabled`,
		rule.RuleID, rule.SecurityGroupID, rule.Direction, rule.Priority, rule.Action, rule.Protocol, nullZeroInt(rule.PortFrom), nullZeroInt(rule.PortTo), rule.PeerType, rule.PeerValue, rule.Description, rule.Enabled, rule.CreatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresSecurityGroupRuleDeleteTxLocked(ctx context.Context, tx *sql.Tx, ruleID string) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `delete from security_group_rules where id=$1`, ruleID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresNetworkCreateTxLocked(ctx context.Context, tx *sql.Tx, network Network, group SecurityGroup, zone NetworkDNSZone) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `insert into networks(id,owner_user_id,name,code,template_key,intra_group_policy,is_default,created_at,updated_at)
		values($1,$2,$3,$4,$5,$6,$7,to_timestamp($8),to_timestamp($9))`,
		network.NetworkID, network.OwnerUserID, network.Name, network.Code, network.TemplateKey, defaultSecurityGroupPolicy(network.IntraGroupPolicy), network.Default, network.CreatedAt, network.UpdatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `insert into security_groups(id,network_id,name,description,status,created_at)
		values($1,$2,$3,$4,$5,to_timestamp($6))`,
		group.SecurityGroupID, group.NetworkID, group.Name, group.Description, group.Status, group.CreatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `insert into network_dns_zones(id,network_id,zone_name,expose_global,status,created_at)
		values($1,$2,$3,$4,$5,to_timestamp($6))`,
		zone.ZoneID, zone.NetworkID, zone.ZoneName, zone.ExposeGlobal, zone.Status, zone.CreatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresNetworkUpdateTxLocked(ctx context.Context, tx *sql.Tx, network Network) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `update networks set name=$1, code=$2, template_key=$3, intra_group_policy=$4, updated_at=to_timestamp($5) where id=$6`,
		network.Name, network.Code, network.TemplateKey, defaultSecurityGroupPolicy(network.IntraGroupPolicy), network.UpdatedAt, network.NetworkID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
