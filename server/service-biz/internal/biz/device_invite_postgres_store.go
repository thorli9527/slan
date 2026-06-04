package biz

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

type postgresDeviceInviteStore struct {
	db *sql.DB
}

func (s *postgresDeviceInviteStore) Save(invite DeviceInvite, ttl time.Duration) error {
	if s == nil || s.db == nil {
		return errNotFound
	}
	if invite.ExpiresAt <= 0 {
		invite.ExpiresAt = time.Now().Add(ttl).Unix()
	}
	_, err := s.db.ExecContext(context.Background(), `insert into device_invites(id,inviter_user_id,invite_code,status,created_at,expires_at,accepted_device_id,accepted_user_id,accepted_at)
		values($1,$2,$3,$4,to_timestamp($5),to_timestamp($6),nullif($7,''),nullif($8,''),to_timestamp(nullif($9,0)))`,
		invite.InviteID, invite.InviterUserID, invite.InviteCode, invite.Status, invite.CreatedAt, invite.ExpiresAt, invite.AcceptedDeviceID, invite.AcceptedUserID, invite.AcceptedAt)
	if err != nil {
		return err
	}
	return nil
}

func (s *postgresDeviceInviteStore) Consume(inviteCode string) (DeviceInvite, error) {
	if s == nil || s.db == nil {
		return DeviceInvite{}, errNotFound
	}
	var invite DeviceInvite
	err := s.db.QueryRowContext(context.Background(), `select id,inviter_user_id,invite_code,status,extract(epoch from created_at)::bigint,extract(epoch from expires_at)::bigint,coalesce(accepted_device_id,''),coalesce(accepted_user_id,''),coalesce(extract(epoch from accepted_at)::bigint,0)
		from device_invites where invite_code=$1 and status='pending' and expires_at > now()`, strings.TrimSpace(inviteCode)).
		Scan(&invite.InviteID, &invite.InviterUserID, &invite.InviteCode, &invite.Status, &invite.CreatedAt, &invite.ExpiresAt, &invite.AcceptedDeviceID, &invite.AcceptedUserID, &invite.AcceptedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DeviceInvite{}, errNotFound
	}
	if err != nil {
		return DeviceInvite{}, err
	}
	return invite, nil
}

func (s *postgresDeviceInviteStore) SaveBootstrapKey(key DeviceBootstrapKey, ttl time.Duration) error {
	if s == nil || s.db == nil {
		return errNotFound
	}
	if key.ExpiresAt <= 0 {
		key.ExpiresAt = time.Now().Add(ttl).Unix()
	}
	_, err := s.db.ExecContext(context.Background(), `insert into device_bootstrap_keys(id,key_hash,created_by_user_id,network_id,device_alias,status,created_at,expires_at,used_at,used_by_device_id,revoked_at)
		values($1,$2,$3,$4,$5,$6,to_timestamp($7),to_timestamp($8),to_timestamp(nullif($9,0)),nullif($10,''),to_timestamp(nullif($11,0)))`,
		key.KeyID, key.KeyHash, key.CreatedByUserID, key.NetworkID, key.DeviceAlias, key.Status, key.CreatedAt, key.ExpiresAt, key.UsedAt, key.UsedByDeviceID, key.RevokedAt)
	if err != nil {
		return err
	}
	return nil
}

func (s *postgresDeviceInviteStore) ConsumeBootstrapKey(keyHash string) (DeviceBootstrapKey, error) {
	if s == nil || s.db == nil {
		return DeviceBootstrapKey{}, errNotFound
	}
	var key DeviceBootstrapKey
	err := s.db.QueryRowContext(context.Background(), `select id,key_hash,created_by_user_id,network_id,coalesce(device_alias,''),status,extract(epoch from created_at)::bigint,extract(epoch from expires_at)::bigint,coalesce(extract(epoch from used_at)::bigint,0),coalesce(used_by_device_id,''),coalesce(extract(epoch from revoked_at)::bigint,0)
		from device_bootstrap_keys where key_hash=$1 and status='unused' and expires_at > now() and revoked_at is null`, strings.TrimSpace(keyHash)).
		Scan(&key.KeyID, &key.KeyHash, &key.CreatedByUserID, &key.NetworkID, &key.DeviceAlias, &key.Status, &key.CreatedAt, &key.ExpiresAt, &key.UsedAt, &key.UsedByDeviceID, &key.RevokedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return DeviceBootstrapKey{}, errNotFound
	}
	if err != nil {
		return DeviceBootstrapKey{}, err
	}
	return key, nil
}
