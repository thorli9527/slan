package biz

import (
	"context"
	"database/sql"
)

func (s *Store) persistPostgresDeviceGroupUpsertTxLocked(ctx context.Context, tx *sql.Tx, group DeviceGroup) error {
	if tx == nil {
		return nil
	}
	_, err := tx.ExecContext(ctx, `insert into device_groups(id,user_id,name,created_at,updated_at)
		values($1,$2,$3,to_timestamp($4),to_timestamp($5))
		on conflict(id) do update set name=excluded.name, updated_at=excluded.updated_at`,
		group.GroupID, group.UserID, group.Name, group.CreatedAt, group.UpdatedAt)
	return err
}

func (s *Store) persistPostgresDeviceGroupDeleteTxLocked(ctx context.Context, tx *sql.Tx, groupID string) error {
	if tx == nil {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `delete from device_group_members where group_id=$1`, groupID); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `delete from device_groups where id=$1`, groupID)
	return err
}

func (s *Store) persistPostgresDeviceGroupMembersForDeviceTxLocked(ctx context.Context, tx *sql.Tx, userID, deviceID string, members []DeviceGroupMember) error {
	if tx == nil {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `delete from device_group_members
		where device_id=$1 and group_id in (select id from device_groups where user_id=$2)`, deviceID, userID); err != nil {
		return err
	}
	for _, member := range members {
		if _, err := tx.ExecContext(ctx, `insert into device_group_members(group_id,device_id,added_at)
			values($1,$2,to_timestamp($3))
			on conflict(group_id,device_id) do update set added_at=excluded.added_at`,
			member.GroupID, member.DeviceID, member.AddedAt); err != nil {
			return err
		}
	}
	return nil
}
