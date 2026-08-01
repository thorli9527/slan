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
	claimed, err := store.ClaimDueNetworkEventDeliveries(context.Background(), 100, 200, 10)
	if err != nil || len(claimed) != 1 || claimed[0].NextRetryAt != 200 {
		t.Fatalf("expected one leased delivery, got %+v err=%v", claimed, err)
	}
	claimedAgain, err := store.ClaimDueNetworkEventDeliveries(context.Background(), 100, 200, 10)
	if err != nil || len(claimedAgain) != 0 {
		t.Fatalf("leased delivery was claimed twice: %+v err=%v", claimedAgain, err)
	}
	loaded, ok, err := store.GetNetworkEventDelivery(context.Background(), "event-1", "device-1")
	if err != nil || !ok || loaded.NetworkID != "network-1" {
		t.Fatalf("unexpected loaded delivery: %+v ok=%t err=%v", loaded, ok, err)
	}
	acknowledged := loaded
	acknowledged.Status = "acknowledged"
	acknowledged.AcknowledgedAt = 101
	acknowledged.UpdatedAt = 101
	if err := store.SaveNetworkEventDelivery(context.Background(), acknowledged); err != nil {
		t.Fatal(err)
	}
	staleRetry := loaded
	staleRetry.Attempts++
	staleRetry.NextRetryAt = 160
	staleRetry.UpdatedAt = 102
	updated, err := store.UpdatePendingNetworkEventDelivery(context.Background(), staleRetry)
	if err != nil {
		t.Fatal(err)
	}
	if updated {
		t.Fatal("stale retry unexpectedly overwrote an acknowledged delivery")
	}
	loaded, ok, err = store.GetNetworkEventDelivery(context.Background(), "event-1", "device-1")
	if err != nil || !ok || loaded.Status != "acknowledged" || loaded.AcknowledgedAt != 101 {
		t.Fatalf("acknowledged delivery changed after stale retry: %+v ok=%t err=%v", loaded, ok, err)
	}
}
