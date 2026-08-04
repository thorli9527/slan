package repository

import (
	"context"
	"strings"
	"testing"

	"github.com/slan/service-biz/internal/model"
)

func TestNetworkEventDeliveryPersistsInDedicatedTable(t *testing.T) {
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
	item := model.NetworkEventDelivery{
		EventID: "event-1", TargetDeviceID: "device-1", NetworkID: "network-1",
		EventType: "member_added", ConfigVersion: 3, Payload: `{}`, Status: "pending",
		Attempts: 1, NextRetryAt: 100, ExpiresAt: 300, CreatedAt: 1, UpdatedAt: 1,
	}
	if err := store.SaveNetworkEventDelivery(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	due, err := store.ListDueNetworkEventDeliveries(context.Background(), 100, 10)
	if err != nil || len(due) != 1 {
		t.Fatalf("expected one due delivery, got %d err=%v", len(due), err)
	}
	loaded, ok, err := store.GetNetworkEventDelivery(context.Background(), "event-1", "device-1")
	if err != nil || !ok || loaded.NetworkID != "network-1" {
		t.Fatalf("unexpected loaded delivery: %+v ok=%t err=%v", loaded, ok, err)
	}
	deleted, err := store.DeleteExpiredNetworkEventDeliveries(context.Background(), 300)
	if err != nil || deleted != 1 {
		t.Fatalf("expected one expired delivery deletion, got deleted=%d err=%v", deleted, err)
	}
}
