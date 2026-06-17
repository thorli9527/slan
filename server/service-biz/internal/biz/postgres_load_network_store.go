package biz

import (
	"context"
)

func (s *Store) loadPostgresNetworksLocked(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `select id,owner_user_id,name,code,coalesce(template_key,''),coalesce(intra_group_policy,'allow'),is_default,extract(epoch from created_at)::bigint,extract(epoch from updated_at)::bigint from networks`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var network Network
		if err := rows.Scan(&network.NetworkID, &network.OwnerUserID, &network.Name, &network.Code, &network.TemplateKey, &network.IntraGroupPolicy, &network.Default, &network.CreatedAt, &network.UpdatedAt); err != nil {
			return err
		}
		network.IntraGroupPolicy = defaultSecurityGroupPolicy(network.IntraGroupPolicy)
		s.networks[network.NetworkID] = network
		s.nextNetworkSeq = maxInt(s.nextNetworkSeq, numericIDSuffix(network.NetworkID)+1)
	}
	return rows.Err()
}

func (s *Store) loadPostgresNetworkDevicesLocked(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `select id,network_id,device_id,owner_user_id,coalesce(alias,''),enabled,status,extract(epoch from created_at)::bigint,extract(epoch from updated_at)::bigint from network_devices`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var membership NetworkDevice
		if err := rows.Scan(&membership.NetworkDeviceID, &membership.NetworkID, &membership.DeviceID, &membership.OwnerUserID, &membership.Alias, &membership.Enabled, &membership.Status, &membership.CreatedAt, &membership.UpdatedAt); err != nil {
			return err
		}
		s.networkDevices[membership.NetworkID+"|"+membership.DeviceID] = membership
	}
	return rows.Err()
}

func (s *Store) loadPostgresNetworkConfigResourcesLocked(ctx context.Context) error {
	zoneRows, err := s.db.QueryContext(ctx, `select id,network_id,zone_name,expose_global,status,extract(epoch from created_at)::bigint from network_dns_zones`)
	if err != nil {
		return err
	}
	defer zoneRows.Close()
	for zoneRows.Next() {
		var zone NetworkDNSZone
		if err := zoneRows.Scan(&zone.ZoneID, &zone.NetworkID, &zone.ZoneName, &zone.ExposeGlobal, &zone.Status, &zone.CreatedAt); err != nil {
			return err
		}
		s.dnsZones[zone.ZoneID] = zone
		s.nextZoneSeq = maxInt(s.nextZoneSeq, numericIDSuffix(zone.ZoneID)+1)
	}
	if err := zoneRows.Err(); err != nil {
		return err
	}
	recordRows, err := s.db.QueryContext(ctx, `select id,zone_id,network_id,name,fqdn,record_type,coalesce(target_device_id,''),coalesce(host(target_ip),''),coalesce(cname,''),coalesce(port,''),ttl,status,extract(epoch from created_at)::bigint from network_dns_records`)
	if err != nil {
		return err
	}
	defer recordRows.Close()
	for recordRows.Next() {
		var record NetworkDNSRecord
		if err := recordRows.Scan(&record.RecordID, &record.ZoneID, &record.NetworkID, &record.Name, &record.FQDN, &record.RecordType, &record.TargetDeviceID, &record.TargetIP, &record.CNAME, &record.Port, &record.TTL, &record.Status, &record.CreatedAt); err != nil {
			return err
		}
		s.dnsRecords[record.RecordID] = record
		s.nextRecordSeq = maxInt(s.nextRecordSeq, numericIDSuffix(record.RecordID)+1)
	}
	if err := recordRows.Err(); err != nil {
		return err
	}
	groupRows, err := s.db.QueryContext(ctx, `select id,network_id,name,coalesce(description,''),status,extract(epoch from created_at)::bigint from security_groups`)
	if err != nil {
		return err
	}
	defer groupRows.Close()
	for groupRows.Next() {
		var group SecurityGroup
		if err := groupRows.Scan(&group.SecurityGroupID, &group.NetworkID, &group.Name, &group.Description, &group.Status, &group.CreatedAt); err != nil {
			return err
		}
		s.securityGroups[group.SecurityGroupID] = group
		s.nextSecuritySeq = maxInt(s.nextSecuritySeq, numericIDSuffix(group.SecurityGroupID)+1)
	}
	if err := groupRows.Err(); err != nil {
		return err
	}
	ruleRows, err := s.db.QueryContext(ctx, `select id,security_group_id,direction,priority,action,protocol,coalesce(port_from,0),coalesce(port_to,0),peer_type,peer_value,coalesce(description,''),enabled,extract(epoch from created_at)::bigint from security_group_rules`)
	if err != nil {
		return err
	}
	defer ruleRows.Close()
	for ruleRows.Next() {
		var rule SecurityGroupRule
		if err := ruleRows.Scan(&rule.RuleID, &rule.SecurityGroupID, &rule.Direction, &rule.Priority, &rule.Action, &rule.Protocol, &rule.PortFrom, &rule.PortTo, &rule.PeerType, &rule.PeerValue, &rule.Description, &rule.Enabled, &rule.CreatedAt); err != nil {
			return err
		}
		s.securityGroupRules[rule.RuleID] = rule
		s.nextSecurityRuleSeq = maxInt(s.nextSecurityRuleSeq, numericIDSuffix(rule.RuleID)+1)
	}
	if err := ruleRows.Err(); err != nil {
		return err
	}
	mappingRows, err := s.db.QueryContext(ctx, `select id,network_id,alias,public_domain,coalesce(source_record,''),device_id,protocol,port,external_port,status,extract(epoch from created_at)::bigint,extract(epoch from updated_at)::bigint from public_domain_mappings`)
	if err != nil {
		return err
	}
	defer mappingRows.Close()
	for mappingRows.Next() {
		var mapping PublicDomainMapping
		if err := mappingRows.Scan(&mapping.MappingID, &mapping.NetworkID, &mapping.Alias, &mapping.PublicDomain, &mapping.SourceRecord, &mapping.DeviceID, &mapping.Protocol, &mapping.Port, &mapping.ExternalPort, &mapping.Status, &mapping.CreatedAt, &mapping.UpdatedAt); err != nil {
			return err
		}
		s.publicMappings[mapping.MappingID] = mapping
		s.nextPublicMapSeq = maxInt(s.nextPublicMapSeq, numericIDSuffix(mapping.MappingID)+1)
	}
	return mappingRows.Err()
}
