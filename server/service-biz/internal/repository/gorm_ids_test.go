package repository

import (
	"fmt"
	"strings"
	"sync"
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

func TestNewDeviceVirtualIPIDIsUniqueUnderConcurrency(t *testing.T) {
	store, err := OpenGormStore(GormConfig{})
	if err != nil {
		if strings.Contains(err.Error(), "failed to connect") || strings.Contains(err.Error(), "dial error") {
			t.Skipf("postgres is not available for repository integration test: %v", err)
		}
		t.Fatalf("open gorm store: %v", err)
	}
	if err := store.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	const count = 32
	ids := make(chan string, count)
	errs := make(chan error, count)
	var wait sync.WaitGroup
	for index := 0; index < count; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			id := store.NewDeviceVirtualIPID()
			if id == "" {
				errs <- fmt.Errorf("empty virtual IP sequence id")
				return
			}
			ids <- id
		}()
	}
	wait.Wait()
	close(ids)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	seen := make(map[string]struct{}, count)
	for id := range ids {
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate virtual IP sequence id: %s", id)
		}
		seen[id] = struct{}{}
	}
	if len(seen) != count {
		t.Fatalf("generated %d unique ids, want %d", len(seen), count)
	}
}
