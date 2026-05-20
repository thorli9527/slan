package biz

import (
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
)

func (s *Store) persistPostgresCoreLocked(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := s.replacePostgresCoreLocked(ctx, tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) beginPostgresCoreWriteLocked(ctx context.Context) (*sql.Tx, error) {
	if s.db == nil {
		return nil, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `select pg_advisory_xact_lock(918273645)`); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	s.resetPostgresCoreStateLocked()
	if err := s.loadPostgresCoreLocked(ctx); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	subnetCount := len(s.ipamSubnets)
	ipCount := len(s.globalIPs)
	s.ensureIPPoolLocked(timeNow().Unix())
	if len(s.ipamSubnets) != subnetCount || len(s.globalIPs) != ipCount {
		if err := s.persistPostgresIPAMPoolTxLocked(ctx, tx); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
	}
	return tx, nil
}

func claimPostgresGlobalIPTx(ctx context.Context, tx *sql.Tx, deviceID, ip string, assignedAt int64) error {
	if tx == nil || strings.TrimSpace(ip) == "" {
		return nil
	}
	result, err := tx.ExecContext(ctx, `update global_ip_addresses
		set device_id=$1,status='assigned',assigned_at=to_timestamp($2),released_at=null
		where ip=$3 and (status='available' or device_id=$1)`,
		deviceID, assignedAt, ip)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if affected != 1 {
		_ = tx.Rollback()
		return errConflict
	}
	return nil
}

func (s *Store) persistPostgresIPAMPoolTxLocked(ctx context.Context, tx *sql.Tx) error {
	for _, subnet := range sortedValues(s.ipamSubnets, func(a, b IPAMSubnet) bool { return a.StartOffset < b.StartOffset }) {
		if _, err := tx.ExecContext(ctx, `insert into ipam_subnets(id,cidr_block,base_ip,prefix_length,start_offset,end_offset,generated_capacity,status,created_at)
			values($1,$2,$3,$4,$5,$6,$7,$8,to_timestamp($9))
			on conflict(id) do update set
				cidr_block=excluded.cidr_block,
				base_ip=excluded.base_ip,
				prefix_length=excluded.prefix_length,
				start_offset=excluded.start_offset,
				end_offset=excluded.end_offset,
				generated_capacity=excluded.generated_capacity,
				status=excluded.status`,
			subnet.SubnetID, subnet.CIDRBlock, subnet.BaseIP, subnet.PrefixLength, subnet.StartOffset, subnet.EndOffset, subnet.GeneratedCapacity, subnet.Status, subnet.CreatedAt); err != nil {
			return err
		}
	}
	for _, address := range sortedValues(s.globalIPs, func(a, b GlobalIPAddress) bool { return a.Offset < b.Offset }) {
		var deviceID any
		if strings.TrimSpace(address.DeviceID) != "" {
			deviceID = address.DeviceID
		}
		if _, err := tx.ExecContext(ctx, `insert into global_ip_addresses(id,subnet_id,ip,address_offset,cidr_block,device_id,status,created_at,assigned_at,released_at)
			values($1,$2,$3,$4,$5,$6,$7,to_timestamp($8),to_timestamp(nullif($9,0)),to_timestamp(nullif($10,0)))
			on conflict(ip) do nothing`,
			address.AddressID, address.SubnetID, address.IP, address.Offset, address.CIDRBlock, deviceID, address.Status, address.CreatedAt, address.AssignedAt, address.ReleasedAt); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) refreshPostgresCoreLocked(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	s.resetPostgresCoreStateLocked()
	if err := s.loadPostgresCoreLocked(ctx); err != nil {
		return err
	}
	s.ensureIPPoolLocked(timeNow().Unix())
	return nil
}

func (s *Store) persistPostgresLoginFailureLocked(ctx context.Context, failure LoginFailure) error {
	if s.db == nil {
		return nil
	}
	var blockedUntil int64
	if failure.BlockedUntil > 0 {
		blockedUntil = failure.BlockedUntil
	}
	_, err := s.db.ExecContext(ctx, `insert into login_failures(key,failed_count,first_failed_at,last_failed_at,blocked_until)
		values($1,$2,to_timestamp($3),to_timestamp($4),to_timestamp(nullif($5,0)))
		on conflict(key) do update set
			failed_count=excluded.failed_count,
			first_failed_at=excluded.first_failed_at,
			last_failed_at=excluded.last_failed_at,
			blocked_until=excluded.blocked_until`,
		failure.Key, failure.FailedCount, failure.FirstFailedAt, failure.LastFailedAt, blockedUntil)
	return err
}

func (s *Store) deletePostgresLoginFailureLocked(ctx context.Context, key string) error {
	if s.db == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `delete from login_failures where key=$1`, key)
	return err
}

func (s *Store) loadPostgresLoginFailureByKeyLocked(ctx context.Context, key string) (LoginFailure, bool, error) {
	if s.db == nil {
		return LoginFailure{}, false, nil
	}
	var failure LoginFailure
	err := s.db.QueryRowContext(ctx, `select key,failed_count,extract(epoch from first_failed_at)::bigint,extract(epoch from last_failed_at)::bigint,coalesce(extract(epoch from blocked_until)::bigint,0)
		from login_failures where key=$1`, key).
		Scan(&failure.Key, &failure.FailedCount, &failure.FirstFailedAt, &failure.LastFailedAt, &failure.BlockedUntil)
	if err == sql.ErrNoRows {
		return LoginFailure{}, false, nil
	}
	if err != nil {
		return LoginFailure{}, false, err
	}
	return failure, true, nil
}

func (s *Store) incrementPostgresLoginFailureLocked(ctx context.Context, key string, now int64, maxFailures int) (LoginFailure, error) {
	if s.db == nil {
		return LoginFailure{}, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return LoginFailure{}, err
	}
	if _, err := tx.ExecContext(ctx, `select pg_advisory_xact_lock(hashtext($1))`, key); err != nil {
		_ = tx.Rollback()
		return LoginFailure{}, err
	}
	var failure LoginFailure
	err = tx.QueryRowContext(ctx, `select key,failed_count,extract(epoch from first_failed_at)::bigint,extract(epoch from last_failed_at)::bigint,coalesce(extract(epoch from blocked_until)::bigint,0)
		from login_failures where key=$1 for update`, key).
		Scan(&failure.Key, &failure.FailedCount, &failure.FirstFailedAt, &failure.LastFailedAt, &failure.BlockedUntil)
	if err == sql.ErrNoRows {
		failure = LoginFailure{Key: key, FirstFailedAt: now}
	} else if err != nil {
		_ = tx.Rollback()
		return LoginFailure{}, err
	}
	if failure.BlockedUntil > now {
		if err := tx.Commit(); err != nil {
			return LoginFailure{}, err
		}
		return failure, nil
	}
	if failure.Key == "" || now-failure.FirstFailedAt > int64(loginRateLimitWindow.Seconds()) {
		failure = LoginFailure{Key: key, FirstFailedAt: now}
	}
	failure.FailedCount++
	failure.LastFailedAt = now
	if failure.FailedCount > maxFailures {
		failure.BlockedUntil = now + int64(loginRateLimitBlock.Seconds())
	} else {
		failure.BlockedUntil = 0
	}
	var blockedUntil int64
	if failure.BlockedUntil > 0 {
		blockedUntil = failure.BlockedUntil
	}
	if _, err := tx.ExecContext(ctx, `insert into login_failures(key,failed_count,first_failed_at,last_failed_at,blocked_until)
		values($1,$2,to_timestamp($3),to_timestamp($4),to_timestamp(nullif($5,0)))
		on conflict(key) do update set
			failed_count=excluded.failed_count,
			first_failed_at=excluded.first_failed_at,
			last_failed_at=excluded.last_failed_at,
			blocked_until=excluded.blocked_until`,
		failure.Key, failure.FailedCount, failure.FirstFailedAt, failure.LastFailedAt, blockedUntil); err != nil {
		_ = tx.Rollback()
		return LoginFailure{}, err
	}
	if err := tx.Commit(); err != nil {
		return LoginFailure{}, err
	}
	return failure, nil
}

func (s *Store) persistPostgresCoreTxLocked(ctx context.Context, tx *sql.Tx) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if err := s.replacePostgresCoreLocked(ctx, tx); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func rollbackPostgresCoreTx(tx *sql.Tx) {
	if tx != nil {
		_ = tx.Rollback()
	}
}

func (s *Store) persistPostgresDeviceRuntimeTxLocked(ctx context.Context, tx *sql.Tx, device Device, status DeviceRuntimeStatus) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `update devices set status=$1, updated_at=to_timestamp($2) where device_id=$3`,
		device.Status, device.UpdatedAt, device.DeviceID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `insert into device_runtime_status(device_id,heartbeat_online,network_enabled,device_enabled,rx_bytes_total,tx_bytes_total,last_seen_at,last_report_at)
		values($1,$2,$3,$4,$5,$6,to_timestamp(nullif($7,0)),to_timestamp(nullif($8,0)))
		on conflict(device_id) do update set
			heartbeat_online=excluded.heartbeat_online,
			network_enabled=excluded.network_enabled,
			device_enabled=excluded.device_enabled,
			rx_bytes_total=excluded.rx_bytes_total,
			tx_bytes_total=excluded.tx_bytes_total,
			last_seen_at=excluded.last_seen_at,
			last_report_at=excluded.last_report_at`,
		status.DeviceID, status.HeartbeatOnline, status.NetworkEnabled, status.DeviceEnabled, status.RxBytesTotal, status.TxBytesTotal, status.LastSeenAt, status.LastReportAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresOpsDeviceUpdateTxLocked(ctx context.Context, tx *sql.Tx, device Device, status DeviceRuntimeStatus, memberships []NetworkDevice) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if err := s.persistPostgresDeviceRuntimeNoCommitLocked(ctx, tx, device, status); err != nil {
		return err
	}
	for _, membership := range memberships {
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
	}
	return tx.Commit()
}

func (s *Store) persistPostgresCustomerProfileUpdateTxLocked(ctx context.Context, tx *sql.Tx, user User, disabled bool, devices []Device, statuses []DeviceRuntimeStatus, memberships []NetworkDevice) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `update users set email=$1, display_name=$2, status=$3, updated_at=to_timestamp($4) where id=$5`,
		user.Email, user.Name, user.Status, user.UpdatedAt, user.UserID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if disabled {
		if _, err := tx.ExecContext(ctx, `delete from user_sessions where user_id=$1`, user.UserID); err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := tx.ExecContext(ctx, `update device_sessions set state='revoked' where user_id=$1 and state='active'`, user.UserID); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	statusByDevice := make(map[string]DeviceRuntimeStatus, len(statuses))
	for _, status := range statuses {
		statusByDevice[status.DeviceID] = status
	}
	for _, device := range devices {
		if err := s.persistPostgresDeviceRuntimeNoCommitLocked(ctx, tx, device, statusByDevice[device.DeviceID]); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	for _, membership := range memberships {
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
	}
	return tx.Commit()
}

func (s *Store) persistPostgresDeviceSessionRenewTxLocked(ctx context.Context, tx *sql.Tx, device Device, status DeviceRuntimeStatus, session DeviceSession) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if err := s.persistPostgresDeviceRuntimeNoCommitLocked(ctx, tx, device, status); err != nil {
		_ = tx.Rollback()
		return err
	}
	activeNetworks, _ := json.Marshal(session.ActiveNetworkIDs)
	if _, err := tx.ExecContext(ctx, `update device_sessions set
			device_token_expires_at=to_timestamp($1),
			active_network_ids=$2,
			state=$3,
			last_renewed_at=to_timestamp($4)
		where id=$5`,
		session.DeviceTokenExpiresAt, string(activeNetworks), session.State, session.LastRenewedAt, session.SessionID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresDeviceRuntimeNoCommitLocked(ctx context.Context, tx *sql.Tx, device Device, status DeviceRuntimeStatus) error {
	if _, err := tx.ExecContext(ctx, `update devices set status=$1, updated_at=to_timestamp($2) where device_id=$3`,
		device.Status, device.UpdatedAt, device.DeviceID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `insert into device_runtime_status(device_id,heartbeat_online,network_enabled,device_enabled,rx_bytes_total,tx_bytes_total,last_seen_at,last_report_at)
		values($1,$2,$3,$4,$5,$6,to_timestamp(nullif($7,0)),to_timestamp(nullif($8,0)))
		on conflict(device_id) do update set
			heartbeat_online=excluded.heartbeat_online,
			network_enabled=excluded.network_enabled,
			device_enabled=excluded.device_enabled,
			rx_bytes_total=excluded.rx_bytes_total,
			tx_bytes_total=excluded.tx_bytes_total,
			last_seen_at=excluded.last_seen_at,
			last_report_at=excluded.last_report_at`,
		status.DeviceID, status.HeartbeatOnline, status.NetworkEnabled, status.DeviceEnabled, status.RxBytesTotal, status.TxBytesTotal, status.LastSeenAt, status.LastReportAt)
	return err
}

func (s *Store) persistPostgresControlDeliveryTxLocked(ctx context.Context, tx *sql.Tx, delivery MQTTControlDelivery) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	messageType := defaultString(strings.TrimSpace(delivery.MessageType), "control")
	expiresAt := delivery.ExpiresAt
	if expiresAt <= 0 {
		expiresAt = maxInt64(delivery.UpdatedAt+int64(defaultControlMessageTTLSeconds), delivery.AckedAt+int64(defaultControlMessageTTLSeconds))
	}
	createdAt := delivery.CreatedAt
	if createdAt <= 0 {
		createdAt = delivery.UpdatedAt
	}
	payloadJSON := string(delivery.Payload)
	if _, err := tx.ExecContext(ctx, `insert into mqtt_control_deliveries(id,delivery_id,device_id,message_type,task_id,action,status,attempt_count,last_error,payload,created_at,expires_at,published_at,processed_at,acked_at,updated_at)
		values($1,$2,$3,$4,$5,$6,$7,$8,$9,nullif($10,'')::jsonb,to_timestamp($11),to_timestamp($12),to_timestamp(nullif($13,0)),to_timestamp(nullif($14,0)),to_timestamp(nullif($15,0)),to_timestamp($16))
		on conflict(device_id, delivery_id) do update set
			message_type=excluded.message_type,
			task_id=excluded.task_id,
			action=excluded.action,
			status=excluded.status,
			attempt_count=excluded.attempt_count,
			last_error=excluded.last_error,
			payload=excluded.payload,
			created_at=excluded.created_at,
			expires_at=excluded.expires_at,
			published_at=excluded.published_at,
			processed_at=excluded.processed_at,
			acked_at=excluded.acked_at,
			updated_at=excluded.updated_at`,
		"mqtt-delivery-"+delivery.DeviceID+"-"+delivery.DeliveryID, delivery.DeliveryID, delivery.DeviceID, messageType, delivery.TaskID, delivery.Action, delivery.Status, delivery.AttemptCount, delivery.Error, payloadJSON, createdAt, expiresAt, delivery.PublishedAt, delivery.ProcessedAtMs/1000, delivery.AckedAt, delivery.UpdatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	return nil
}

func (s *Store) claimPostgresControlDeliveryRetryLocked(ctx context.Context, deviceID, deliveryID string, previousUpdatedAt, now int64) (bool, error) {
	result, err := s.db.ExecContext(ctx, `update mqtt_control_deliveries
		set status='retrying', updated_at=to_timestamp($1)
		where device_id=$2
			and delivery_id=$3
			and updated_at=to_timestamp($4)
			and status in ('pending','retrying','published')
			and acked_at is null
			and expires_at > to_timestamp($1)`,
		now, deviceID, deliveryID, previousUpdatedAt)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

func (s *Store) persistPostgresUserSessionTxLocked(ctx context.Context, tx *sql.Tx, session UserSession) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `insert into user_sessions(id,user_id,refresh_token_hash,access_token,client_type,expires_at,created_at)
		values($1,$2,$3,$4,'web',to_timestamp($5),to_timestamp($6))
		on conflict(id) do update set
			refresh_token_hash=excluded.refresh_token_hash,
			access_token=excluded.access_token,
			expires_at=excluded.expires_at`,
		session.SessionID, session.UserID, session.Token, session.Token, session.ExpiresAt, session.CreatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresUserRegisterTxLocked(ctx context.Context, tx *sql.Tx, user User, network Network, group SecurityGroup, zone NetworkDNSZone, session UserSession) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `insert into users(id,email,password_hash,display_name,status,created_at,updated_at)
		values($1,$2,$3,$4,$5,to_timestamp($6),to_timestamp($7))`,
		user.UserID, user.Email, user.PasswordHash, user.Name, user.Status, user.CreatedAt, user.UpdatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `insert into networks(id,owner_user_id,name,code,template_key,status,is_default,created_at,updated_at)
		values($1,$2,$3,$4,$5,$6,$7,to_timestamp($8),to_timestamp($9))`,
		network.NetworkID, network.OwnerUserID, network.Name, network.Code, network.TemplateKey, network.Status, network.Default, network.CreatedAt, network.UpdatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `insert into security_groups(id,network_id,name,description,default_policy,status,created_at)
		values($1,$2,$3,$4,$5,$6,to_timestamp($7))`,
		group.SecurityGroupID, group.NetworkID, group.Name, group.Description, group.DefaultPolicy, group.Status, group.CreatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `insert into network_dns_zones(id,network_id,zone_name,expose_global,status,created_at)
		values($1,$2,$3,$4,$5,to_timestamp($6))`,
		zone.ZoneID, zone.NetworkID, zone.ZoneName, zone.ExposeGlobal, zone.Status, zone.CreatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `insert into user_sessions(id,user_id,refresh_token_hash,access_token,client_type,expires_at,created_at)
		values($1,$2,$3,$4,'web',to_timestamp($5),to_timestamp($6))`,
		session.SessionID, session.UserID, session.Token, session.Token, session.ExpiresAt, session.CreatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresUserSessionRenewTxLocked(ctx context.Context, tx *sql.Tx, session UserSession) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `update user_sessions set expires_at=to_timestamp($1) where access_token=$2`,
		session.ExpiresAt, session.Token); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresConsoleLoginKeyTxLocked(ctx context.Context, tx *sql.Tx, key ConsoleLoginKey) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `insert into console_login_keys(login_key,user_id,device_id,status,created_at,expires_at,consumed_at)
		values($1,$2,$3,$4,to_timestamp($5),to_timestamp($6),to_timestamp(nullif($7,0)))
		on conflict(login_key) do update set
			status=excluded.status,
			consumed_at=excluded.consumed_at`,
		key.LoginKey, key.UserID, key.DeviceID, key.Status, key.CreatedAt, key.ExpiresAt, key.ConsumedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresConsoleLoginConsumeTxLocked(ctx context.Context, tx *sql.Tx, key ConsoleLoginKey, session UserSession) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `update console_login_keys set status=$1, consumed_at=to_timestamp($2) where login_key=$3`,
		key.Status, key.ConsumedAt, key.LoginKey); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `insert into user_sessions(id,user_id,refresh_token_hash,access_token,client_type,expires_at,created_at)
		values($1,$2,$3,$4,'web',to_timestamp($5),to_timestamp($6))`,
		session.SessionID, session.UserID, session.Token, session.Token, session.ExpiresAt, session.CreatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresLogoutTxLocked(ctx context.Context, tx *sql.Tx, accessToken, deviceToken, userID string) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if strings.TrimSpace(accessToken) != "" {
		if _, err := tx.ExecContext(ctx, `delete from user_sessions where access_token=$1`, strings.TrimSpace(accessToken)); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if strings.TrimSpace(userID) != "" {
		if _, err := tx.ExecContext(ctx, `update device_sessions set state='revoked' where user_id=$1 and state='active'`, strings.TrimSpace(userID)); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if strings.TrimSpace(deviceToken) != "" {
		if _, err := tx.ExecContext(ctx, `update device_sessions set state='revoked' where device_token=$1`, strings.TrimSpace(deviceToken)); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) persistPostgresDeviceAliasTxLocked(ctx context.Context, tx *sql.Tx, device Device) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `update devices set alias=$1, updated_at=to_timestamp($2) where device_id=$3`,
		device.Alias, device.UpdatedAt, device.DeviceID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresUserAliasTxLocked(ctx context.Context, tx *sql.Tx, alias UserAlias) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	aliasID := "alias-" + alias.OwnerUserID + "-" + alias.Email
	if _, err := tx.ExecContext(ctx, `insert into user_aliases(id,owner_user_id,email,alias,updated_at)
		values($1,$2,$3,$4,to_timestamp($5))
		on conflict(owner_user_id,email) do update set
			alias=excluded.alias,
			updated_at=excluded.updated_at`,
		aliasID, alias.OwnerUserID, alias.Email, alias.Alias, alias.UpdatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresBootstrapKeyRevokeTxLocked(ctx context.Context, tx *sql.Tx, key DeviceBootstrapKey) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `update device_bootstrap_keys set status=$1, revoked_at=to_timestamp(nullif($2,0)) where id=$3`,
		key.Status, key.RevokedAt, key.KeyID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresUserPasswordTxLocked(ctx context.Context, tx *sql.Tx, user User) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `update users set password_hash=$1, updated_at=to_timestamp($2) where id=$3`,
		user.PasswordHash, user.UpdatedAt, user.UserID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

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
	if _, err := tx.ExecContext(ctx, `insert into security_groups(id,network_id,name,description,default_policy,status,created_at)
		values($1,$2,$3,$4,$5,$6,to_timestamp($7))
		on conflict(id) do update set
			name=excluded.name,
			description=excluded.description,
			default_policy=excluded.default_policy,
			status=excluded.status`,
		group.SecurityGroupID, group.NetworkID, group.Name, group.Description, group.DefaultPolicy, group.Status, group.CreatedAt); err != nil {
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
	if _, err := tx.ExecContext(ctx, `insert into networks(id,owner_user_id,name,code,template_key,status,is_default,created_at,updated_at)
		values($1,$2,$3,$4,$5,$6,$7,to_timestamp($8),to_timestamp($9))`,
		network.NetworkID, network.OwnerUserID, network.Name, network.Code, network.TemplateKey, network.Status, network.Default, network.CreatedAt, network.UpdatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `insert into security_groups(id,network_id,name,description,default_policy,status,created_at)
		values($1,$2,$3,$4,$5,$6,to_timestamp($7))`,
		group.SecurityGroupID, group.NetworkID, group.Name, group.Description, group.DefaultPolicy, group.Status, group.CreatedAt); err != nil {
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
	if _, err := tx.ExecContext(ctx, `update networks set name=$1, code=$2, template_key=$3, status=$4, updated_at=to_timestamp($5) where id=$6`,
		network.Name, network.Code, network.TemplateKey, network.Status, network.UpdatedAt, network.NetworkID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresDeviceInviteAcceptTxLocked(ctx context.Context, tx *sql.Tx, invite DeviceInvite, grant DeviceAccessGrant) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `update device_invites set status=$1, accepted_device_id=$2, accepted_user_id=$3, accepted_at=to_timestamp($4) where invite_code=$5`,
		invite.Status, invite.AcceptedDeviceID, invite.AcceptedUserID, invite.AcceptedAt, invite.InviteCode); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `insert into device_access_grants(id,device_id,user_id,granted_by,invite_code,status,created_at)
		values($1,$2,$3,nullif($4,''),$5,$6,to_timestamp($7))
		on conflict(device_id,user_id) do update set
			granted_by=excluded.granted_by,
			invite_code=excluded.invite_code,
			status=excluded.status`,
		grant.GrantID, grant.DeviceID, grant.UserID, grant.GrantedBy, grant.InviteCode, grant.Status, grant.CreatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresDeviceAccessGrantRevokeTxLocked(ctx context.Context, tx *sql.Tx, grant DeviceAccessGrant) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `update device_access_grants set status=$1 where id=$2`, grant.Status, grant.GrantID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresBootstrapDeviceSessionTxLocked(ctx context.Context, tx *sql.Tx, device Device, membership NetworkDevice, session DeviceSession, key DeviceBootstrapKey) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `insert into devices(id,owner_user_id,device_id,name,platform,os_name,os_version,alias,public_key,status,created_at,updated_at)
		values($1,nullif($2,''),$3,$4,$5,$6,$7,$8,$9,$10,to_timestamp($11),to_timestamp($12))
		on conflict(device_id) do update set
			owner_user_id=excluded.owner_user_id,
			name=excluded.name,
			platform=excluded.platform,
			os_name=excluded.os_name,
			os_version=excluded.os_version,
			alias=excluded.alias,
			public_key=excluded.public_key,
			status=excluded.status,
			updated_at=excluded.updated_at`,
		device.DeviceID, device.OwnerID, device.DeviceID, device.Name, device.Platform, device.OSName, device.OSVersion, device.Alias, device.PublicKey, device.Status, device.CreatedAt, device.UpdatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	if strings.TrimSpace(device.GlobalIP) != "" {
		if err := claimPostgresGlobalIPTx(ctx, tx, device.DeviceID, device.GlobalIP, device.UpdatedAt); err != nil {
			return err
		}
	}
	for _, entry := range s.ownerLogs {
		if entry.DeviceID != device.DeviceID {
			continue
		}
		if _, err := tx.ExecContext(ctx, `insert into device_owner_change_logs(id,device_id,from_user_id,to_user_id,reason,changed_at)
			values($1,$2,nullif($3,''),$4,$5,to_timestamp($6))
			on conflict(id) do nothing`, entry.LogID, entry.DeviceID, entry.FromUserID, entry.ToUserID, entry.Reason, entry.ChangedAt); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if strings.TrimSpace(membership.NetworkID) != "" && strings.TrimSpace(membership.DeviceID) != "" {
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
	}
	status := s.runtimeStatuses[device.DeviceID]
	if status.DeviceID != "" {
		if _, err := tx.ExecContext(ctx, `insert into device_runtime_status(device_id,heartbeat_online,network_enabled,device_enabled,rx_bytes_total,tx_bytes_total,last_seen_at,last_report_at)
			values($1,$2,$3,$4,$5,$6,to_timestamp(nullif($7,0)),to_timestamp(nullif($8,0)))
			on conflict(device_id) do update set
				heartbeat_online=excluded.heartbeat_online,
				network_enabled=excluded.network_enabled,
				device_enabled=excluded.device_enabled,
				rx_bytes_total=excluded.rx_bytes_total,
				tx_bytes_total=excluded.tx_bytes_total,
				last_seen_at=excluded.last_seen_at,
				last_report_at=excluded.last_report_at`,
			status.DeviceID, status.HeartbeatOnline, status.NetworkEnabled, status.DeviceEnabled, status.RxBytesTotal, status.TxBytesTotal, status.LastSeenAt, status.LastReportAt); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	activeNetworks, _ := json.Marshal(session.ActiveNetworkIDs)
	if _, err := tx.ExecContext(ctx, `insert into device_sessions(id,device_id,user_id,device_token,device_token_expires_at,device_refresh_token,active_network_ids,state,registered_at,last_renewed_at)
		values($1,$2,$3,$4,to_timestamp($5),$6,$7,$8,to_timestamp($9),to_timestamp($10))`,
		session.SessionID, session.DeviceID, session.UserID, session.DeviceToken, session.DeviceTokenExpiresAt, session.DeviceRefreshToken, string(activeNetworks), session.State, session.RegisteredAt, session.LastRenewedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	if _, err := tx.ExecContext(ctx, `update device_bootstrap_keys set status=$1, used_at=to_timestamp(nullif($2,0)), used_by_device_id=$3 where id=$4`,
		key.Status, key.UsedAt, key.UsedByDeviceID, key.KeyID); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresDeviceBindSessionTxLocked(ctx context.Context, tx *sql.Tx, device Device, membership NetworkDevice, session DeviceSession, previousOwnerID string) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `insert into devices(id,owner_user_id,device_id,name,platform,os_name,os_version,alias,public_key,status,created_at,updated_at)
		values($1,nullif($2,''),$3,$4,$5,$6,$7,$8,$9,$10,to_timestamp($11),to_timestamp($12))
		on conflict(device_id) do update set
			owner_user_id=excluded.owner_user_id,
			name=excluded.name,
			platform=excluded.platform,
			os_name=excluded.os_name,
			os_version=excluded.os_version,
			alias=excluded.alias,
			public_key=excluded.public_key,
			status=excluded.status,
			updated_at=excluded.updated_at`,
		device.DeviceID, device.OwnerID, device.DeviceID, device.Name, device.Platform, device.OSName, device.OSVersion, device.Alias, device.PublicKey, device.Status, device.CreatedAt, device.UpdatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	if strings.TrimSpace(device.GlobalIP) != "" {
		if err := claimPostgresGlobalIPTx(ctx, tx, device.DeviceID, device.GlobalIP, device.UpdatedAt); err != nil {
			return err
		}
	}
	if strings.TrimSpace(previousOwnerID) != "" && previousOwnerID != device.OwnerID {
		if _, err := tx.ExecContext(ctx, `delete from network_devices where device_id=$1 and owner_user_id=$2`, device.DeviceID, previousOwnerID); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	for _, entry := range s.ownerLogs {
		if entry.DeviceID != device.DeviceID {
			continue
		}
		if _, err := tx.ExecContext(ctx, `insert into device_owner_change_logs(id,device_id,from_user_id,to_user_id,reason,changed_at)
			values($1,$2,nullif($3,''),$4,$5,to_timestamp($6))
			on conflict(id) do nothing`, entry.LogID, entry.DeviceID, entry.FromUserID, entry.ToUserID, entry.Reason, entry.ChangedAt); err != nil {
			_ = tx.Rollback()
			return err
		}
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
	status := s.runtimeStatuses[device.DeviceID]
	if status.DeviceID != "" {
		if _, err := tx.ExecContext(ctx, `insert into device_runtime_status(device_id,heartbeat_online,network_enabled,device_enabled,rx_bytes_total,tx_bytes_total,last_seen_at,last_report_at)
			values($1,$2,$3,$4,$5,$6,to_timestamp(nullif($7,0)),to_timestamp(nullif($8,0)))
			on conflict(device_id) do update set
				heartbeat_online=excluded.heartbeat_online,
				network_enabled=excluded.network_enabled,
				device_enabled=excluded.device_enabled,
				rx_bytes_total=excluded.rx_bytes_total,
				tx_bytes_total=excluded.tx_bytes_total,
				last_seen_at=excluded.last_seen_at,
				last_report_at=excluded.last_report_at`,
			status.DeviceID, status.HeartbeatOnline, status.NetworkEnabled, status.DeviceEnabled, status.RxBytesTotal, status.TxBytesTotal, status.LastSeenAt, status.LastReportAt); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if _, err := tx.ExecContext(ctx, `update device_sessions set state='revoked' where device_id=$1 and state='active'`, device.DeviceID); err != nil {
		_ = tx.Rollback()
		return err
	}
	activeNetworks, _ := json.Marshal(session.ActiveNetworkIDs)
	if _, err := tx.ExecContext(ctx, `insert into device_sessions(id,device_id,user_id,device_token,device_token_expires_at,device_refresh_token,active_network_ids,state,registered_at,last_renewed_at)
		values($1,$2,$3,$4,to_timestamp($5),$6,$7,$8,to_timestamp($9),to_timestamp($10))`,
		session.SessionID, session.DeviceID, session.UserID, session.DeviceToken, session.DeviceTokenExpiresAt, session.DeviceRefreshToken, string(activeNetworks), session.State, session.RegisteredAt, session.LastRenewedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Store) persistPostgresDeviceRegisterTxLocked(ctx context.Context, tx *sql.Tx, device Device, membership NetworkDevice, previousOwnerID string) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if _, err := tx.ExecContext(ctx, `insert into devices(id,owner_user_id,device_id,name,platform,os_name,os_version,alias,public_key,status,created_at,updated_at)
		values($1,nullif($2,''),$3,$4,$5,$6,$7,$8,$9,$10,to_timestamp($11),to_timestamp($12))
		on conflict(device_id) do update set
			owner_user_id=excluded.owner_user_id,
			name=excluded.name,
			platform=excluded.platform,
			os_name=excluded.os_name,
			os_version=excluded.os_version,
			alias=excluded.alias,
			public_key=excluded.public_key,
			status=excluded.status,
			updated_at=excluded.updated_at`,
		device.DeviceID, device.OwnerID, device.DeviceID, device.Name, device.Platform, device.OSName, device.OSVersion, device.Alias, device.PublicKey, device.Status, device.CreatedAt, device.UpdatedAt); err != nil {
		_ = tx.Rollback()
		return err
	}
	if strings.TrimSpace(device.GlobalIP) != "" {
		if err := claimPostgresGlobalIPTx(ctx, tx, device.DeviceID, device.GlobalIP, device.UpdatedAt); err != nil {
			return err
		}
	}
	if strings.TrimSpace(previousOwnerID) != "" && previousOwnerID != device.OwnerID {
		if _, err := tx.ExecContext(ctx, `delete from network_devices where device_id=$1 and owner_user_id=$2`, device.DeviceID, previousOwnerID); err != nil {
			_ = tx.Rollback()
			return err
		}
		if _, err := tx.ExecContext(ctx, `update device_sessions set state='revoked' where device_id=$1 and user_id=$2 and state='active'`, device.DeviceID, previousOwnerID); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	for _, entry := range s.ownerLogs {
		if entry.DeviceID != device.DeviceID {
			continue
		}
		if _, err := tx.ExecContext(ctx, `insert into device_owner_change_logs(id,device_id,from_user_id,to_user_id,reason,changed_at)
			values($1,$2,nullif($3,''),$4,$5,to_timestamp($6))
			on conflict(id) do nothing`, entry.LogID, entry.DeviceID, entry.FromUserID, entry.ToUserID, entry.Reason, entry.ChangedAt); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	if strings.TrimSpace(membership.NetworkID) != "" && strings.TrimSpace(membership.DeviceID) != "" {
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
	}
	status := s.runtimeStatuses[device.DeviceID]
	if status.DeviceID != "" {
		if _, err := tx.ExecContext(ctx, `insert into device_runtime_status(device_id,heartbeat_online,network_enabled,device_enabled,rx_bytes_total,tx_bytes_total,last_seen_at,last_report_at)
			values($1,$2,$3,$4,$5,$6,to_timestamp(nullif($7,0)),to_timestamp(nullif($8,0)))
			on conflict(device_id) do update set
				heartbeat_online=excluded.heartbeat_online,
				network_enabled=excluded.network_enabled,
				device_enabled=excluded.device_enabled,
				rx_bytes_total=excluded.rx_bytes_total,
				tx_bytes_total=excluded.tx_bytes_total,
				last_seen_at=excluded.last_seen_at,
				last_report_at=excluded.last_report_at`,
			status.DeviceID, status.HeartbeatOnline, status.NetworkEnabled, status.DeviceEnabled, status.RxBytesTotal, status.TxBytesTotal, status.LastSeenAt, status.LastReportAt); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) persistPostgresDeviceDeleteTxLocked(ctx context.Context, tx *sql.Tx, deviceID string) error {
	if tx == nil {
		return s.persistPostgresCoreLocked(ctx)
	}
	if err := s.deletePostgresDeviceTxLocked(ctx, tx, deviceID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) deletePostgresDeviceTxLocked(ctx context.Context, tx *sql.Tx, deviceID string) error {
	if tx == nil {
		return nil
	}
	for _, statement := range []string{
		`delete from mqtt_control_deliveries where device_id=$1`,
		`delete from device_sessions where device_id=$1`,
		`delete from network_config_versions where device_id=$1`,
		`delete from network_devices where device_id=$1`,
		`delete from device_runtime_status where device_id=$1`,
		`delete from device_access_grants where device_id=$1`,
		`update global_ip_addresses set device_id=null,status='available',released_at=now() where device_id=$1`,
		`delete from devices where device_id=$1`,
	} {
		if _, err := tx.ExecContext(ctx, statement, deviceID); err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	return nil
}

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

func (s *Store) loadPostgresCoreLocked(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	if err := s.loadPostgresUsersLocked(ctx); err != nil {
		return err
	}
	if err := s.loadPostgresNetworksLocked(ctx); err != nil {
		return err
	}
	if err := s.loadPostgresDevicesLocked(ctx); err != nil {
		return err
	}
	if err := s.loadPostgresLoginFailuresLocked(ctx); err != nil {
		return err
	}
	if err := s.loadPostgresDeviceSharingLocked(ctx); err != nil {
		return err
	}
	if err := s.loadPostgresIPAMLocked(ctx); err != nil {
		return err
	}
	if err := s.loadPostgresNetworkDevicesLocked(ctx); err != nil {
		return err
	}
	if err := s.loadPostgresRuntimeStatusesLocked(ctx); err != nil {
		return err
	}
	if err := s.loadPostgresNetworkConfigResourcesLocked(ctx); err != nil {
		return err
	}
	if err := s.loadPostgresDeviceSessionsLocked(ctx); err != nil {
		return err
	}
	return s.loadPostgresControlDeliveriesLocked(ctx)
}

func (s *Store) loadPostgresUsersLocked(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `select id,email,password_hash,coalesce(display_name,''),status,extract(epoch from created_at)::bigint,extract(epoch from updated_at)::bigint from users`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.UserID, &user.Email, &user.PasswordHash, &user.Name, &user.Status, &user.CreatedAt, &user.UpdatedAt); err != nil {
			return err
		}
		s.users[user.UserID] = user
		s.userByEmail[user.Email] = user.UserID
		s.nextUserID = maxInt(s.nextUserID, numericIDSuffix(user.UserID)+1)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	sessionRows, err := s.db.QueryContext(ctx, `select id,user_id,access_token,extract(epoch from created_at)::bigint,extract(epoch from expires_at)::bigint from user_sessions`)
	if err != nil {
		return err
	}
	defer sessionRows.Close()
	for sessionRows.Next() {
		var session UserSession
		if err := sessionRows.Scan(&session.SessionID, &session.UserID, &session.Token, &session.CreatedAt, &session.ExpiresAt); err != nil {
			return err
		}
		if session.ExpiresAt > timeNow().Unix() {
			s.sessions[session.Token] = session
		}
		s.nextUserSessionSeq = maxInt(s.nextUserSessionSeq, numericIDSuffix(session.SessionID)+1)
	}
	if err := sessionRows.Err(); err != nil {
		return err
	}

	keyRows, err := s.db.QueryContext(ctx, `select login_key,user_id,coalesce(device_id,''),status,extract(epoch from created_at)::bigint,extract(epoch from expires_at)::bigint,coalesce(extract(epoch from consumed_at)::bigint,0) from console_login_keys`)
	if err != nil {
		return err
	}
	defer keyRows.Close()
	for keyRows.Next() {
		var key ConsoleLoginKey
		if err := keyRows.Scan(&key.LoginKey, &key.UserID, &key.DeviceID, &key.Status, &key.CreatedAt, &key.ExpiresAt, &key.ConsumedAt); err != nil {
			return err
		}
		if key.Status == "unused" && key.ExpiresAt <= timeNow().Unix() {
			key.Status = "expired"
		}
		s.consoleLoginKeys[key.LoginKey] = key
	}
	return keyRows.Err()
}

func (s *Store) loadPostgresLoginFailuresLocked(ctx context.Context) error {
	now := timeNow().Unix()
	rows, err := s.db.QueryContext(ctx, `select key,failed_count,extract(epoch from first_failed_at)::bigint,extract(epoch from last_failed_at)::bigint,coalesce(extract(epoch from blocked_until)::bigint,0)
		from login_failures
		where first_failed_at > to_timestamp($1) or blocked_until > to_timestamp($2)`,
		now-int64(loginRateLimitWindow.Seconds()), now)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var failure LoginFailure
		if err := rows.Scan(&failure.Key, &failure.FailedCount, &failure.FirstFailedAt, &failure.LastFailedAt, &failure.BlockedUntil); err != nil {
			return err
		}
		s.loginFailures[failure.Key] = failure
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `delete from login_failures where first_failed_at <= to_timestamp($1) and (blocked_until is null or blocked_until <= to_timestamp($2))`,
		now-int64(loginRateLimitWindow.Seconds()), now)
	return err
}

func (s *Store) loadPostgresNetworksLocked(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `select id,owner_user_id,name,code,coalesce(template_key,''),status,is_default,extract(epoch from created_at)::bigint,extract(epoch from updated_at)::bigint from networks`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var network Network
		if err := rows.Scan(&network.NetworkID, &network.OwnerUserID, &network.Name, &network.Code, &network.TemplateKey, &network.Status, &network.Default, &network.CreatedAt, &network.UpdatedAt); err != nil {
			return err
		}
		s.networks[network.NetworkID] = network
		s.nextNetworkSeq = maxInt(s.nextNetworkSeq, numericIDSuffix(network.NetworkID)+1)
	}
	return rows.Err()
}

func (s *Store) loadPostgresDevicesLocked(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `select device_id,coalesce(owner_user_id,''),coalesce(name,''),platform,coalesce(os_name,''),coalesce(os_version,''),coalesce(alias,''),coalesce(public_key,''),status,extract(epoch from created_at)::bigint,extract(epoch from updated_at)::bigint from devices`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var device Device
		if err := rows.Scan(&device.DeviceID, &device.OwnerID, &device.Name, &device.Platform, &device.OSName, &device.OSVersion, &device.Alias, &device.PublicKey, &device.Status, &device.CreatedAt, &device.UpdatedAt); err != nil {
			return err
		}
		device.GlobalName = sanitizeDNSLabel(device.DeviceID) + "." + globalDeviceDomain()
		s.devices[device.DeviceID] = device
		if strings.TrimSpace(device.OwnerID) != "" {
			s.addDeviceOwnerLocked(device.DeviceID, device.OwnerID, maxInt64(device.CreatedAt, device.UpdatedAt))
		}
	}
	return rows.Err()
}

func (s *Store) loadPostgresDeviceSharingLocked(ctx context.Context) error {
	aliasRows, err := s.db.QueryContext(ctx, `select owner_user_id,email,alias,extract(epoch from updated_at)::bigint from user_aliases`)
	if err != nil {
		return err
	}
	defer aliasRows.Close()
	for aliasRows.Next() {
		var alias UserAlias
		if err := aliasRows.Scan(&alias.OwnerUserID, &alias.Email, &alias.Alias, &alias.UpdatedAt); err != nil {
			return err
		}
		s.userAliases[alias.OwnerUserID+"|"+alias.Email] = alias
	}
	if err := aliasRows.Err(); err != nil {
		return err
	}

	logRows, err := s.db.QueryContext(ctx, `select id,device_id,coalesce(from_user_id,''),to_user_id,reason,extract(epoch from changed_at)::bigint from device_owner_change_logs`)
	if err != nil {
		return err
	}
	defer logRows.Close()
	for logRows.Next() {
		var entry DeviceOwnerChangeLog
		if err := logRows.Scan(&entry.LogID, &entry.DeviceID, &entry.FromUserID, &entry.ToUserID, &entry.Reason, &entry.ChangedAt); err != nil {
			return err
		}
		s.ownerLogs[entry.LogID] = entry
		s.nextOwnerLogSeq = maxInt(s.nextOwnerLogSeq, numericIDSuffix(entry.LogID)+1)
	}
	if err := logRows.Err(); err != nil {
		return err
	}

	inviteRows, err := s.db.QueryContext(ctx, `select id,inviter_user_id,invite_code,status,extract(epoch from created_at)::bigint,extract(epoch from expires_at)::bigint,coalesce(accepted_device_id,''),coalesce(accepted_user_id,''),coalesce(extract(epoch from accepted_at)::bigint,0) from device_invites`)
	if err != nil {
		return err
	}
	defer inviteRows.Close()
	for inviteRows.Next() {
		var invite DeviceInvite
		if err := inviteRows.Scan(&invite.InviteID, &invite.InviterUserID, &invite.InviteCode, &invite.Status, &invite.CreatedAt, &invite.ExpiresAt, &invite.AcceptedDeviceID, &invite.AcceptedUserID, &invite.AcceptedAt); err != nil {
			return err
		}
		s.deviceInvites[invite.InviteCode] = invite
		s.nextDeviceInviteSeq = maxInt(s.nextDeviceInviteSeq, numericIDSuffix(invite.InviteID)+1)
	}
	if err := inviteRows.Err(); err != nil {
		return err
	}

	grantRows, err := s.db.QueryContext(ctx, `select id,device_id,user_id,coalesce(granted_by,''),coalesce(invite_code,''),status,extract(epoch from created_at)::bigint from device_access_grants`)
	if err != nil {
		return err
	}
	defer grantRows.Close()
	for grantRows.Next() {
		var grant DeviceAccessGrant
		if err := grantRows.Scan(&grant.GrantID, &grant.DeviceID, &grant.UserID, &grant.GrantedBy, &grant.InviteCode, &grant.Status, &grant.CreatedAt); err != nil {
			return err
		}
		s.deviceAccessGrants[grant.UserID+"|"+grant.DeviceID] = grant
		s.nextDeviceInviteSeq = maxInt(s.nextDeviceInviteSeq, numericIDSuffix(grant.GrantID)+1)
	}
	if err := grantRows.Err(); err != nil {
		return err
	}

	keyRows, err := s.db.QueryContext(ctx, `select id,key_hash,created_by_user_id,network_id,coalesce(device_alias,''),status,extract(epoch from created_at)::bigint,extract(epoch from expires_at)::bigint,coalesce(extract(epoch from used_at)::bigint,0),coalesce(used_by_device_id,''),coalesce(extract(epoch from revoked_at)::bigint,0) from device_bootstrap_keys`)
	if err != nil {
		return err
	}
	defer keyRows.Close()
	for keyRows.Next() {
		var key DeviceBootstrapKey
		if err := keyRows.Scan(&key.KeyID, &key.KeyHash, &key.CreatedByUserID, &key.NetworkID, &key.DeviceAlias, &key.Status, &key.CreatedAt, &key.ExpiresAt, &key.UsedAt, &key.UsedByDeviceID, &key.RevokedAt); err != nil {
			return err
		}
		s.deviceBootstrapKeys[key.KeyID] = key
		s.nextBootstrapKeySeq = maxInt(s.nextBootstrapKeySeq, numericIDSuffix(key.KeyID)+1)
	}
	return keyRows.Err()
}

func (s *Store) loadPostgresIPAMLocked(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `select id,cidr_block::text,host(base_ip),prefix_length,start_offset,end_offset,generated_capacity,status,extract(epoch from created_at)::bigint from ipam_subnets`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var subnet IPAMSubnet
		if err := rows.Scan(&subnet.SubnetID, &subnet.CIDRBlock, &subnet.BaseIP, &subnet.PrefixLength, &subnet.StartOffset, &subnet.EndOffset, &subnet.GeneratedCapacity, &subnet.Status, &subnet.CreatedAt); err != nil {
			return err
		}
		s.ipamSubnets[subnet.SubnetID] = subnet
		s.nextIPSubnetSeq = maxInt(s.nextIPSubnetSeq, numericIDSuffix(subnet.SubnetID)+1)
		s.nextIPOffset = maxUint32(s.nextIPOffset, subnet.EndOffset+1)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	addressRows, err := s.db.QueryContext(ctx, `select id,subnet_id,host(ip),cidr_block::text,address_offset,coalesce(device_id,''),status,extract(epoch from created_at)::bigint,coalesce(extract(epoch from assigned_at)::bigint,0),coalesce(extract(epoch from released_at)::bigint,0) from global_ip_addresses`)
	if err != nil {
		return err
	}
	defer addressRows.Close()
	for addressRows.Next() {
		var address GlobalIPAddress
		if err := addressRows.Scan(&address.AddressID, &address.SubnetID, &address.IP, &address.CIDRBlock, &address.Offset, &address.DeviceID, &address.Status, &address.CreatedAt, &address.AssignedAt, &address.ReleasedAt); err != nil {
			return err
		}
		s.globalIPs[address.IP] = address
		if device, ok := s.devices[address.DeviceID]; ok {
			device.GlobalIP = address.IP
			s.devices[device.DeviceID] = device
		}
		s.nextIPAddressSeq = maxInt(s.nextIPAddressSeq, numericIDSuffix(address.AddressID)+1)
	}
	return addressRows.Err()
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

func (s *Store) loadPostgresRuntimeStatusesLocked(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `select device_id,heartbeat_online,network_enabled,device_enabled,rx_bytes_total,tx_bytes_total,coalesce(extract(epoch from last_seen_at)::bigint,0),coalesce(extract(epoch from last_report_at)::bigint,0) from device_runtime_status`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var status DeviceRuntimeStatus
		if err := rows.Scan(&status.DeviceID, &status.HeartbeatOnline, &status.NetworkEnabled, &status.DeviceEnabled, &status.RxBytesTotal, &status.TxBytesTotal, &status.LastSeenAt, &status.LastReportAt); err != nil {
			return err
		}
		s.runtimeStatuses[status.DeviceID] = status
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
	groupRows, err := s.db.QueryContext(ctx, `select id,network_id,name,coalesce(description,''),default_policy,status,extract(epoch from created_at)::bigint from security_groups`)
	if err != nil {
		return err
	}
	defer groupRows.Close()
	for groupRows.Next() {
		var group SecurityGroup
		if err := groupRows.Scan(&group.SecurityGroupID, &group.NetworkID, &group.Name, &group.Description, &group.DefaultPolicy, &group.Status, &group.CreatedAt); err != nil {
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

func (s *Store) loadPostgresDeviceSessionsLocked(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `select id,device_id,user_id,device_token,extract(epoch from device_token_expires_at)::bigint,device_refresh_token,active_network_ids::text,state,extract(epoch from registered_at)::bigint,extract(epoch from last_renewed_at)::bigint from device_sessions`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var session DeviceSession
		var activeNetworkJSON string
		if err := rows.Scan(&session.SessionID, &session.DeviceID, &session.UserID, &session.DeviceToken, &session.DeviceTokenExpiresAt, &session.DeviceRefreshToken, &activeNetworkJSON, &session.State, &session.RegisteredAt, &session.LastRenewedAt); err != nil {
			return err
		}
		_ = json.Unmarshal([]byte(activeNetworkJSON), &session.ActiveNetworkIDs)
		if session.State == "active" && session.DeviceTokenExpiresAt > timeNow().Unix() {
			s.deviceSessions[session.SessionID] = session
			s.deviceSessionByToken[session.DeviceToken] = session.SessionID
		}
		s.nextDeviceSessionSeq = maxInt(s.nextDeviceSessionSeq, numericIDSuffix(session.SessionID)+1)
	}
	return rows.Err()
}

func (s *Store) loadPostgresControlDeliveriesLocked(ctx context.Context) error {
	rows, err := s.db.QueryContext(ctx, `select delivery_id,device_id,message_type,coalesce(task_id,''),coalesce(action,''),status,attempt_count,coalesce(last_error,''),coalesce(payload::text,''),extract(epoch from created_at)::bigint,extract(epoch from expires_at)::bigint,coalesce(extract(epoch from published_at)::bigint,0),coalesce(extract(epoch from processed_at)::bigint,0),coalesce(extract(epoch from acked_at)::bigint,0),extract(epoch from updated_at)::bigint from mqtt_control_deliveries`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var delivery MQTTControlDelivery
		var processedAtSeconds int64
		var payloadText string
		if err := rows.Scan(&delivery.DeliveryID, &delivery.DeviceID, &delivery.MessageType, &delivery.TaskID, &delivery.Action, &delivery.Status, &delivery.AttemptCount, &delivery.Error, &payloadText, &delivery.CreatedAt, &delivery.ExpiresAt, &delivery.PublishedAt, &processedAtSeconds, &delivery.AckedAt, &delivery.UpdatedAt); err != nil {
			return err
		}
		if strings.TrimSpace(payloadText) != "" {
			delivery.Payload = json.RawMessage(payloadText)
		}
		delivery.ProcessedAtMs = processedAtSeconds * 1000
		s.controlDeliveries[controlDeliveryKey(delivery.DeviceID, delivery.DeliveryID)] = delivery
	}
	return rows.Err()
}

func (s *Store) loadPostgresControlDeliveryByIDLocked(ctx context.Context, deliveryID string) (MQTTControlDelivery, bool, error) {
	if s.db == nil {
		return MQTTControlDelivery{}, false, nil
	}
	rows, err := s.db.QueryContext(ctx, `select delivery_id,device_id,message_type,coalesce(task_id,''),coalesce(action,''),status,attempt_count,coalesce(last_error,''),coalesce(payload::text,''),extract(epoch from created_at)::bigint,extract(epoch from expires_at)::bigint,coalesce(extract(epoch from published_at)::bigint,0),coalesce(extract(epoch from processed_at)::bigint,0),coalesce(extract(epoch from acked_at)::bigint,0),extract(epoch from updated_at)::bigint from mqtt_control_deliveries where delivery_id=$1 order by updated_at desc limit 1`, deliveryID)
	if err != nil {
		return MQTTControlDelivery{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return MQTTControlDelivery{}, false, rows.Err()
	}
	delivery, err := scanPostgresControlDelivery(rows)
	if err != nil {
		return MQTTControlDelivery{}, false, err
	}
	s.controlDeliveries[controlDeliveryKey(delivery.DeviceID, delivery.DeliveryID)] = delivery
	return delivery, true, rows.Err()
}

func (s *Store) loadPostgresControlDeliveryForDeviceLocked(ctx context.Context, deviceID, deliveryID string) (MQTTControlDelivery, bool, error) {
	if s.db == nil {
		return MQTTControlDelivery{}, false, nil
	}
	row := s.db.QueryRowContext(ctx, `select delivery_id,device_id,message_type,coalesce(task_id,''),coalesce(action,''),status,attempt_count,coalesce(last_error,''),coalesce(payload::text,''),extract(epoch from created_at)::bigint,extract(epoch from expires_at)::bigint,coalesce(extract(epoch from published_at)::bigint,0),coalesce(extract(epoch from processed_at)::bigint,0),coalesce(extract(epoch from acked_at)::bigint,0),extract(epoch from updated_at)::bigint from mqtt_control_deliveries where device_id=$1 and delivery_id=$2`, deviceID, deliveryID)
	delivery, err := scanPostgresControlDelivery(row)
	if err == sql.ErrNoRows {
		return MQTTControlDelivery{}, false, nil
	}
	if err != nil {
		return MQTTControlDelivery{}, false, err
	}
	s.controlDeliveries[controlDeliveryKey(delivery.DeviceID, delivery.DeliveryID)] = delivery
	return delivery, true, nil
}

type postgresControlDeliveryScanner interface {
	Scan(dest ...any) error
}

func scanPostgresControlDelivery(scanner postgresControlDeliveryScanner) (MQTTControlDelivery, error) {
	var delivery MQTTControlDelivery
	var processedAtSeconds int64
	var payloadText string
	if err := scanner.Scan(&delivery.DeliveryID, &delivery.DeviceID, &delivery.MessageType, &delivery.TaskID, &delivery.Action, &delivery.Status, &delivery.AttemptCount, &delivery.Error, &payloadText, &delivery.CreatedAt, &delivery.ExpiresAt, &delivery.PublishedAt, &processedAtSeconds, &delivery.AckedAt, &delivery.UpdatedAt); err != nil {
		return MQTTControlDelivery{}, err
	}
	if strings.TrimSpace(payloadText) != "" {
		delivery.Payload = json.RawMessage(payloadText)
	}
	delivery.ProcessedAtMs = processedAtSeconds * 1000
	return delivery, nil
}

func numericIDSuffix(id string) int {
	index := strings.LastIndex(id, "-")
	if index < 0 || index+1 >= len(id) {
		return 0
	}
	value, _ := strconv.Atoi(id[index+1:])
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func maxUint32(a, b uint32) uint32 {
	if a > b {
		return a
	}
	return b
}

func nullZeroInt(value int) any {
	if value == 0 {
		return nil
	}
	return value
}
