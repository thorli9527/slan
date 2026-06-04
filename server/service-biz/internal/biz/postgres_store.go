package biz

import (
	"context"
	"database/sql"
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
