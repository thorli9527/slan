package biz

import (
	"context"
	"database/sql"
	"encoding/json"
)

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
