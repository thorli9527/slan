package bootstrap

import (
	"context"
	"time"

	"github.com/slan/service-biz/internal/model"
	"github.com/slan/service-biz/internal/repository"
)

const (
	initializeLockID int64 = 2026062001
	backfillLockID   int64 = 2026062002
)

func InitializeGormStore(ctx context.Context, store *repository.GormStore) error {
	return store.Transaction(func(tx *repository.GormStore) error {
		if err := tx.AcquireAdvisoryLock(initializeLockID); err != nil {
			return err
		}
		if err := tx.Migrate(); err != nil {
			return err
		}
		count, err := tx.UserCount(ctx)
		if err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		return seedReferenceDefaults(ctx, tx, time.Now().Unix())
	})
}

func BackfillReferenceDefaults(ctx context.Context, store *repository.GormStore) error {
	return store.Transaction(func(tx *repository.GormStore) error {
		if err := tx.AcquireAdvisoryLock(backfillLockID); err != nil {
			return err
		}
		if err := tx.DeleteLegacyDemoSeed(ctx); err != nil {
			return err
		}
		if err := seedReferenceDefaults(ctx, tx, time.Now().Unix()); err != nil {
			return err
		}
		return tx.ResetReferenceSequences()
	})
}

func seedReferenceDefaults(ctx context.Context, store *repository.GormStore, now int64) error {
	admin := defaultOperator(now)
	if err := store.SaveOperator(ctx, admin); err != nil {
		return err
	}
	if err := seedOpsCatalog(ctx, store, now); err != nil {
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

func seedOpsCatalog(ctx context.Context, store *repository.GormStore, now int64) error {
	if err := store.SaveRelayNode(ctx, defaultRelayNode(now)); err != nil {
		return err
	}
	if punchNode, ok := defaultPunchNode(now); ok {
		if err := store.SavePunchNode(ctx, punchNode); err != nil {
			return err
		}
	} else {
		if err := store.DeletePunchNode(ctx, "punch-000001"); err != nil {
			return err
		}
	}
	for _, plan := range defaultPlans(now) {
		if err := store.SavePlan(ctx, plan); err != nil {
			return err
		}
	}
	for _, product := range defaultProducts(now) {
		if err := store.SaveProduct(ctx, product); err != nil {
			return err
		}
	}
	for _, item := range defaultDownloads(now) {
		if err := store.SaveClientDownload(ctx, item); err != nil {
			return err
		}
	}
	return nil
}

