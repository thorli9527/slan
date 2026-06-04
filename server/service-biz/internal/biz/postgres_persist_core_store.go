package biz

import (
	"context"
	"database/sql"
)

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
