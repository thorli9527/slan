package biz

import (
	"context"
	"encoding/json"
	"strings"
)

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

func (s *Store) loadPostgresDeviceGroupsLocked(ctx context.Context) error {
	groupRows, err := s.db.QueryContext(ctx, `select id,user_id,name,extract(epoch from created_at)::bigint,extract(epoch from updated_at)::bigint from device_groups`)
	if err != nil {
		return err
	}
	defer groupRows.Close()
	for groupRows.Next() {
		var group DeviceGroup
		if err := groupRows.Scan(&group.GroupID, &group.UserID, &group.Name, &group.CreatedAt, &group.UpdatedAt); err != nil {
			return err
		}
		s.deviceGroups[group.GroupID] = group
		s.nextDeviceGroupSeq = maxInt(s.nextDeviceGroupSeq, numericIDSuffix(group.GroupID)+1)
	}
	if err := groupRows.Err(); err != nil {
		return err
	}
	memberRows, err := s.db.QueryContext(ctx, `select group_id,device_id,extract(epoch from added_at)::bigint from device_group_members`)
	if err != nil {
		return err
	}
	defer memberRows.Close()
	for memberRows.Next() {
		var member DeviceGroupMember
		if err := memberRows.Scan(&member.GroupID, &member.DeviceID, &member.AddedAt); err != nil {
			return err
		}
		s.deviceGroupMembers[member.GroupID+"|"+member.DeviceID] = member
	}
	return memberRows.Err()
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
