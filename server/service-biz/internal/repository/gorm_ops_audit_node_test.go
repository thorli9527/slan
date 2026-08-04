package repository

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/slan/service-biz/internal/model"
)

func TestListAuditEventsOrdersByCreationTime(t *testing.T) {
	store, err := OpenGormStore(GormConfig{})
	if err != nil {
		if strings.Contains(err.Error(), "failed to connect") || strings.Contains(err.Error(), "dial error") {
			t.Skipf("postgres is not available for repository integration test: %v", err)
		}
		t.Fatal(err)
	}
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}

	prefix := fmt.Sprintf("audit-order-%d-", time.Now().UnixNano())
	defer store.db.Where("event_id LIKE ?", prefix+"%").Delete(&gormAuditEventRecord{})
	for _, event := range []model.AuditEvent{
		{EventID: prefix + "z", Action: "old", CreatedAt: 4_000_000_001},
		{EventID: prefix + "a", Action: "new-a", CreatedAt: 4_000_000_002},
		{EventID: prefix + "b", Action: "new-b", CreatedAt: 4_000_000_002},
	} {
		if err := store.SaveAuditEvent(context.Background(), event); err != nil {
			t.Fatal(err)
		}
	}

	items, err := store.ListAuditEvents(context.Background(), 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || items[0].EventID != prefix+"b" || items[1].EventID != prefix+"a" || items[2].EventID != prefix+"z" {
		t.Fatalf("unexpected audit event order: %#v", items)
	}
}
