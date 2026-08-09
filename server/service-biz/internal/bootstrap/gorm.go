package bootstrap

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

const (
	initializeLockID int64 = 2026062001
)

func InitializeGormStore(ctx context.Context, store *repository.GormStore) error {
	return store.Transaction(func(tx *repository.GormStore) error {
		if err := tx.AcquireAdvisoryLock(initializeLockID); err != nil {
			return err
		}
		if err := tx.Migrate(); err != nil {
			return err
		}
		cutoff, enabled, err := auditRetentionCutoff(time.Now())
		if err != nil {
			return err
		}
		if enabled {
			if err := tx.DeleteAuditEventsBefore(ctx, cutoff); err != nil {
				return err
			}
		}
		count, err := tx.OperatorCount(ctx)
		if err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		return seedReferenceDefaults(ctx, tx, time.Now().Unix())
	})
}

func auditRetentionCutoff(now time.Time) (int64, bool, error) {
	raw := strings.TrimSpace(os.Getenv("SLAN_AUDIT_RETENTION_DAYS"))
	if raw == "" || raw == "0" {
		return 0, false, nil
	}
	days, err := strconv.Atoi(raw)
	if err != nil || days < 0 || days > 3650 {
		return 0, false, fmt.Errorf("SLAN_AUDIT_RETENTION_DAYS must be between 0 and 3650")
	}
	return now.Add(-time.Duration(days) * 24 * time.Hour).Unix(), true, nil
}

func seedReferenceDefaults(ctx context.Context, store *repository.GormStore, now int64) error {
	admin, err := defaultOperator(now)
	if err != nil {
		return err
	}
	if err := store.SaveOperator(ctx, admin); err != nil {
		return err
	}
	if err := store.SaveAuditEvent(ctx, model.AuditEvent{
		EventID:      "audit-000001",
		ActorType:    "operator",
		ActorID:      admin.OperatorID,
		Action:       "bootstrap",
		ResourceType: "gorm_store",
		ResourceID:   "seed",
		Status:       "success",
		CreatedAt:    now,
	}); err != nil {
		return err
	}
	for _, counter := range defaultSeedCounters() {
		if err := store.SaveCounter(counter.Name, counter.Value); err != nil {
			return err
		}
	}
	return nil
}
