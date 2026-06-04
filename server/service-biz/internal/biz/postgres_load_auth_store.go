package biz

import (
	"context"
)

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
