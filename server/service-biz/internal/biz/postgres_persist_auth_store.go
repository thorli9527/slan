package biz

import (
	"context"
	"database/sql"
	"strings"
)

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
