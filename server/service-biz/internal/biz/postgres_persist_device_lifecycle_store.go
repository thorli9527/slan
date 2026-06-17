package biz

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

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
		`delete from device_group_members where device_id=$1`,
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
