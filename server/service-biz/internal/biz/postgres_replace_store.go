package biz

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

func (s *Store) replacePostgresCoreLocked(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `select pg_advisory_xact_lock(918273645)`); err != nil {
		return err
	}
	for _, statement := range []string{
		`delete from mqtt_control_deliveries`,
		`delete from device_sessions`,
		`delete from device_access_grants`,
		`delete from device_invites`,
		`delete from device_bootstrap_keys`,
		`delete from device_owner_change_logs`,
		`delete from user_aliases`,
		`delete from console_login_keys`,
		`delete from network_devices`,
		`delete from device_runtime_status`,
		`delete from network_dns_records`,
		`delete from public_domain_mappings`,
		`delete from security_group_rules`,
		`delete from global_ip_addresses`,
		`delete from devices`,
		`delete from user_sessions`,
		`delete from network_dns_zones`,
		`delete from security_groups`,
		`delete from networks`,
		`delete from users`,
		`delete from ipam_subnets`,
	} {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	for _, user := range sortedValues(s.users, func(a, b User) bool { return a.UserID < b.UserID }) {
		if _, err := tx.ExecContext(ctx, `insert into users(id,email,password_hash,display_name,status,created_at,updated_at)
			values($1,$2,$3,$4,$5,to_timestamp($6),to_timestamp($7))`,
			user.UserID, user.Email, user.PasswordHash, user.Name, user.Status, user.CreatedAt, user.UpdatedAt); err != nil {
			return err
		}
	}
	for _, alias := range sortedValues(s.userAliases, func(a, b UserAlias) bool {
		if a.OwnerUserID == b.OwnerUserID {
			return a.Email < b.Email
		}
		return a.OwnerUserID < b.OwnerUserID
	}) {
		aliasID := "alias-" + alias.OwnerUserID + "-" + alias.Email
		if _, err := tx.ExecContext(ctx, `insert into user_aliases(id,owner_user_id,email,alias,updated_at)
			values($1,$2,$3,$4,to_timestamp($5))`, aliasID, alias.OwnerUserID, alias.Email, alias.Alias, alias.UpdatedAt); err != nil {
			return err
		}
	}
	for _, network := range sortedValues(s.networks, func(a, b Network) bool { return a.NetworkID < b.NetworkID }) {
		if _, err := tx.ExecContext(ctx, `insert into networks(id,owner_user_id,name,code,template_key,status,is_default,created_at,updated_at)
			values($1,$2,$3,$4,$5,$6,$7,to_timestamp($8),to_timestamp($9))`,
			network.NetworkID, network.OwnerUserID, network.Name, network.Code, network.TemplateKey, network.Status, network.Default, network.CreatedAt, network.UpdatedAt); err != nil {
			return err
		}
	}
	for _, zone := range sortedValues(s.dnsZones, func(a, b NetworkDNSZone) bool { return a.ZoneID < b.ZoneID }) {
		if _, err := tx.ExecContext(ctx, `insert into network_dns_zones(id,network_id,zone_name,expose_global,status,created_at)
			values($1,$2,$3,$4,$5,to_timestamp($6))`, zone.ZoneID, zone.NetworkID, zone.ZoneName, zone.ExposeGlobal, zone.Status, zone.CreatedAt); err != nil {
			return err
		}
	}
	for _, group := range sortedValues(s.securityGroups, func(a, b SecurityGroup) bool { return a.SecurityGroupID < b.SecurityGroupID }) {
		if _, err := tx.ExecContext(ctx, `insert into security_groups(id,network_id,name,description,default_policy,status,created_at)
			values($1,$2,$3,$4,$5,$6,to_timestamp($7))`, group.SecurityGroupID, group.NetworkID, group.Name, group.Description, group.DefaultPolicy, group.Status, group.CreatedAt); err != nil {
			return err
		}
	}
	for _, session := range sortedValues(s.sessions, func(a, b UserSession) bool { return a.SessionID < b.SessionID }) {
		if _, err := tx.ExecContext(ctx, `insert into user_sessions(id,user_id,refresh_token_hash,access_token,client_type,expires_at,created_at)
			values($1,$2,$3,$4,'web',to_timestamp($5),to_timestamp($6))`, session.SessionID, session.UserID, session.Token, session.Token, session.ExpiresAt, session.CreatedAt); err != nil {
			return err
		}
	}
	for _, key := range sortedValues(s.consoleLoginKeys, func(a, b ConsoleLoginKey) bool { return a.LoginKey < b.LoginKey }) {
		if _, err := tx.ExecContext(ctx, `insert into console_login_keys(login_key,user_id,device_id,status,created_at,expires_at,consumed_at)
			values($1,$2,$3,$4,to_timestamp($5),to_timestamp($6),to_timestamp(nullif($7,0)))`,
			key.LoginKey, key.UserID, key.DeviceID, key.Status, key.CreatedAt, key.ExpiresAt, key.ConsumedAt); err != nil {
			return err
		}
	}
	for _, subnet := range sortedValues(s.ipamSubnets, func(a, b IPAMSubnet) bool { return a.StartOffset < b.StartOffset }) {
		if _, err := tx.ExecContext(ctx, `insert into ipam_subnets(id,cidr_block,base_ip,prefix_length,start_offset,end_offset,generated_capacity,status,created_at)
			values($1,$2,$3,$4,$5,$6,$7,$8,to_timestamp($9))`, subnet.SubnetID, subnet.CIDRBlock, subnet.BaseIP, subnet.PrefixLength, subnet.StartOffset, subnet.EndOffset, subnet.GeneratedCapacity, subnet.Status, subnet.CreatedAt); err != nil {
			return err
		}
	}
	for _, device := range sortedValues(s.devices, func(a, b Device) bool { return a.DeviceID < b.DeviceID }) {
		if _, err := tx.ExecContext(ctx, `insert into devices(id,owner_user_id,device_id,name,platform,os_name,os_version,alias,public_key,status,created_at,updated_at)
			values($1,nullif($2,''),$3,$4,$5,$6,$7,$8,$9,$10,to_timestamp($11),to_timestamp($12))`,
			device.DeviceID, device.OwnerID, device.DeviceID, device.Name, device.Platform, device.OSName, device.OSVersion, device.Alias, device.PublicKey, device.Status, device.CreatedAt, device.UpdatedAt); err != nil {
			return err
		}
	}
	for _, log := range sortedValues(s.ownerLogs, func(a, b DeviceOwnerChangeLog) bool { return a.LogID < b.LogID }) {
		if _, err := tx.ExecContext(ctx, `insert into device_owner_change_logs(id,device_id,from_user_id,to_user_id,reason,changed_at)
			values($1,$2,nullif($3,''),$4,$5,to_timestamp($6))`, log.LogID, log.DeviceID, log.FromUserID, log.ToUserID, log.Reason, log.ChangedAt); err != nil {
			return err
		}
	}
	for _, invite := range sortedValues(s.deviceInvites, func(a, b DeviceInvite) bool { return a.InviteID < b.InviteID }) {
		if _, err := tx.ExecContext(ctx, `insert into device_invites(id,inviter_user_id,invite_code,status,created_at,expires_at,accepted_device_id,accepted_user_id,accepted_at)
			values($1,$2,$3,$4,to_timestamp($5),to_timestamp($6),nullif($7,''),nullif($8,''),to_timestamp(nullif($9,0)))`,
			invite.InviteID, invite.InviterUserID, invite.InviteCode, invite.Status, invite.CreatedAt, invite.ExpiresAt, invite.AcceptedDeviceID, invite.AcceptedUserID, invite.AcceptedAt); err != nil {
			return err
		}
	}
	for _, grant := range sortedValues(s.deviceAccessGrants, func(a, b DeviceAccessGrant) bool { return a.GrantID < b.GrantID }) {
		if _, err := tx.ExecContext(ctx, `insert into device_access_grants(id,device_id,user_id,granted_by,invite_code,status,created_at)
			values($1,$2,$3,nullif($4,''),$5,$6,to_timestamp($7))`, grant.GrantID, grant.DeviceID, grant.UserID, grant.GrantedBy, grant.InviteCode, grant.Status, grant.CreatedAt); err != nil {
			return err
		}
	}
	for _, key := range sortedValues(s.deviceBootstrapKeys, func(a, b DeviceBootstrapKey) bool { return a.KeyID < b.KeyID }) {
		if _, err := tx.ExecContext(ctx, `insert into device_bootstrap_keys(id,key_hash,created_by_user_id,network_id,device_alias,status,created_at,expires_at,used_at,used_by_device_id,revoked_at)
			values($1,$2,$3,$4,$5,$6,to_timestamp($7),to_timestamp($8),to_timestamp(nullif($9,0)),nullif($10,''),to_timestamp(nullif($11,0)))`,
			key.KeyID, key.KeyHash, key.CreatedByUserID, key.NetworkID, key.DeviceAlias, key.Status, key.CreatedAt, key.ExpiresAt, key.UsedAt, key.UsedByDeviceID, key.RevokedAt); err != nil {
			return err
		}
	}
	for _, address := range sortedValues(s.globalIPs, func(a, b GlobalIPAddress) bool { return a.Offset < b.Offset }) {
		var deviceID any
		if strings.TrimSpace(address.DeviceID) != "" {
			deviceID = address.DeviceID
		}
		if _, err := tx.ExecContext(ctx, `insert into global_ip_addresses(id,subnet_id,ip,address_offset,cidr_block,device_id,status,created_at,assigned_at,released_at)
			values($1,$2,$3,$4,$5,$6,$7,to_timestamp($8),to_timestamp(nullif($9,0)),to_timestamp(nullif($10,0)))`,
			address.AddressID, address.SubnetID, address.IP, address.Offset, address.CIDRBlock, deviceID, address.Status, address.CreatedAt, address.AssignedAt, address.ReleasedAt); err != nil {
			return err
		}
	}
	for _, membership := range sortedValues(s.networkDevices, func(a, b NetworkDevice) bool { return a.NetworkDeviceID < b.NetworkDeviceID }) {
		if _, err := tx.ExecContext(ctx, `insert into network_devices(id,network_id,device_id,owner_user_id,alias,enabled,status,created_at,updated_at)
			values($1,$2,$3,$4,$5,$6,$7,to_timestamp($8),to_timestamp($9))`,
			membership.NetworkDeviceID, membership.NetworkID, membership.DeviceID, membership.OwnerUserID, membership.Alias, membership.Enabled, membership.Status, membership.CreatedAt, membership.UpdatedAt); err != nil {
			return err
		}
	}
	for _, status := range sortedValues(s.runtimeStatuses, func(a, b DeviceRuntimeStatus) bool { return a.DeviceID < b.DeviceID }) {
		if _, err := tx.ExecContext(ctx, `insert into device_runtime_status(device_id,heartbeat_online,network_enabled,device_enabled,rx_bytes_total,tx_bytes_total,last_seen_at,last_report_at)
			values($1,$2,$3,$4,$5,$6,to_timestamp(nullif($7,0)),to_timestamp(nullif($8,0)))`,
			status.DeviceID, status.HeartbeatOnline, status.NetworkEnabled, status.DeviceEnabled, status.RxBytesTotal, status.TxBytesTotal, status.LastSeenAt, status.LastReportAt); err != nil {
			return err
		}
	}
	for _, record := range sortedValues(s.dnsRecords, func(a, b NetworkDNSRecord) bool { return a.RecordID < b.RecordID }) {
		var targetDeviceID any
		if strings.TrimSpace(record.TargetDeviceID) != "" {
			targetDeviceID = record.TargetDeviceID
		}
		var targetIP any
		if strings.TrimSpace(record.TargetIP) != "" {
			targetIP = record.TargetIP
		}
		if _, err := tx.ExecContext(ctx, `insert into network_dns_records(id,zone_id,network_id,name,fqdn,record_type,target_device_id,target_ip,cname,port,ttl,status,created_at)
			values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,to_timestamp($13))`,
			record.RecordID, record.ZoneID, record.NetworkID, record.Name, record.FQDN, record.RecordType, targetDeviceID, targetIP, record.CNAME, record.Port, record.TTL, record.Status, record.CreatedAt); err != nil {
			return err
		}
	}
	for _, mapping := range sortedValues(s.publicMappings, func(a, b PublicDomainMapping) bool { return a.MappingID < b.MappingID }) {
		if _, err := tx.ExecContext(ctx, `insert into public_domain_mappings(id,network_id,alias,public_domain,source_record,device_id,protocol,port,external_port,status,created_at,updated_at)
			values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,to_timestamp($11),to_timestamp($12))`,
			mapping.MappingID, mapping.NetworkID, mapping.Alias, mapping.PublicDomain, mapping.SourceRecord, mapping.DeviceID, mapping.Protocol, mapping.Port, mapping.ExternalPort, mapping.Status, mapping.CreatedAt, mapping.UpdatedAt); err != nil {
			return err
		}
	}
	for _, rule := range sortedValues(s.securityGroupRules, func(a, b SecurityGroupRule) bool { return a.RuleID < b.RuleID }) {
		if _, err := tx.ExecContext(ctx, `insert into security_group_rules(id,security_group_id,direction,priority,action,protocol,port_from,port_to,peer_type,peer_value,description,enabled,created_at)
			values($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,to_timestamp($13))`,
			rule.RuleID, rule.SecurityGroupID, rule.Direction, rule.Priority, rule.Action, rule.Protocol, nullZeroInt(rule.PortFrom), nullZeroInt(rule.PortTo), rule.PeerType, rule.PeerValue, rule.Description, rule.Enabled, rule.CreatedAt); err != nil {
			return err
		}
	}
	for _, session := range sortedValues(s.deviceSessions, func(a, b DeviceSession) bool { return a.SessionID < b.SessionID }) {
		activeNetworks, _ := json.Marshal(session.ActiveNetworkIDs)
		if _, err := tx.ExecContext(ctx, `insert into device_sessions(id,device_id,user_id,device_token,device_token_expires_at,device_refresh_token,active_network_ids,state,registered_at,last_renewed_at)
			values($1,$2,$3,$4,to_timestamp($5),$6,$7,$8,to_timestamp($9),to_timestamp($10))`,
			session.SessionID, session.DeviceID, session.UserID, session.DeviceToken, session.DeviceTokenExpiresAt, session.DeviceRefreshToken, string(activeNetworks), session.State, session.RegisteredAt, session.LastRenewedAt); err != nil {
			return err
		}
	}
	for _, delivery := range sortedValues(s.controlDeliveries, func(a, b MQTTControlDelivery) bool {
		if a.DeviceID == b.DeviceID {
			return a.DeliveryID < b.DeliveryID
		}
		return a.DeviceID < b.DeviceID
	}) {
		messageType := defaultString(strings.TrimSpace(delivery.MessageType), "control")
		createdAt := delivery.CreatedAt
		if createdAt <= 0 {
			createdAt = delivery.UpdatedAt
		}
		expiresAt := delivery.ExpiresAt
		if expiresAt <= 0 {
			expiresAt = maxInt64(delivery.UpdatedAt+int64(defaultControlMessageTTLSeconds), delivery.AckedAt+int64(defaultControlMessageTTLSeconds))
		}
		payloadJSON := string(delivery.Payload)
		if _, err := tx.ExecContext(ctx, `insert into mqtt_control_deliveries(id,delivery_id,device_id,message_type,task_id,action,status,attempt_count,last_error,payload,created_at,expires_at,published_at,processed_at,acked_at,updated_at)
			values($1,$2,$3,$4,$5,$6,$7,$8,$9,nullif($10,'')::jsonb,to_timestamp($11),to_timestamp($12),to_timestamp(nullif($13,0)),to_timestamp(nullif($14,0)),to_timestamp(nullif($15,0)),to_timestamp($16))`,
			"mqtt-delivery-"+delivery.DeviceID+"-"+delivery.DeliveryID, delivery.DeliveryID, delivery.DeviceID, messageType, delivery.TaskID, delivery.Action, delivery.Status, delivery.AttemptCount, delivery.Error, payloadJSON, createdAt, expiresAt, delivery.PublishedAt, delivery.ProcessedAtMs/1000, delivery.AckedAt, delivery.UpdatedAt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) resetPostgresCoreStateLocked() {
	s.nextUserID = 1
	s.nextUserSessionSeq = 1
	s.nextDeviceSeq = 1
	s.nextNetworkSeq = 1
	s.nextDeviceSessionSeq = 1
	s.nextOwnerSeq = 1
	s.nextOwnerLogSeq = 1
	s.nextZoneSeq = 1
	s.nextRecordSeq = 1
	s.nextSecuritySeq = 1
	s.nextSecurityRuleSeq = 1
	s.nextPublicMapSeq = 1
	s.nextIPSubnetSeq = 1
	s.nextIPAddressSeq = 1
	s.nextIPOffset = 0
	s.users = make(map[string]User)
	s.userByEmail = make(map[string]string)
	s.userAliases = make(map[string]UserAlias)
	s.sessions = make(map[string]UserSession)
	s.loginFailures = make(map[string]LoginFailure)
	s.consoleLoginKeys = make(map[string]ConsoleLoginKey)
	s.devices = make(map[string]Device)
	s.deviceOwners = make(map[string]DeviceOwner)
	s.ownerLogs = make(map[string]DeviceOwnerChangeLog)
	s.ipamSubnets = make(map[string]IPAMSubnet)
	s.globalIPs = make(map[string]GlobalIPAddress)
	s.networks = make(map[string]Network)
	s.deviceInvites = make(map[string]DeviceInvite)
	s.deviceAccessGrants = make(map[string]DeviceAccessGrant)
	s.deviceBootstrapKeys = make(map[string]DeviceBootstrapKey)
	s.networkDevices = make(map[string]NetworkDevice)
	s.dnsZones = make(map[string]NetworkDNSZone)
	s.dnsRecords = make(map[string]NetworkDNSRecord)
	s.publicMappings = make(map[string]PublicDomainMapping)
	s.securityGroups = make(map[string]SecurityGroup)
	s.securityGroupRules = make(map[string]SecurityGroupRule)
	s.runtimeStatuses = make(map[string]DeviceRuntimeStatus)
	s.deviceSessions = make(map[string]DeviceSession)
	s.deviceSessionByToken = make(map[string]string)
	s.controlDeliveries = make(map[string]MQTTControlDelivery)
}
