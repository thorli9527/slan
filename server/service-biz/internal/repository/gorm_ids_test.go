package repository

import (
	"strings"
	"testing"
)

func TestNewDeviceVirtualIPIDUsesSequentialCounter(t *testing.T) {
	store, err := OpenGormStore(GormConfig{DSN: "file::memory:?cache=shared"})
	if err != nil {
		if strings.Contains(err.Error(), "failed to connect") ||
			strings.Contains(err.Error(), "dial error") {
			t.Skipf("postgres is not available for repository integration test: %v", err)
		}
		t.Fatalf("open gorm store: %v", err)
	}
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	first := store.NewDeviceVirtualIPID()
	second := store.NewDeviceVirtualIPID()

	if first != "vip-000001" {
		t.Fatalf("expected first virtual ip id vip-000001, got %q", first)
	}
	if second != "vip-000002" {
		t.Fatalf("expected second virtual ip id vip-000002, got %q", second)
	}
}
