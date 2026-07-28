package service

import (
	"strings"
	"sync"
	"testing"
)

func TestServerMQTTClientIDIsUniqueAcrossConcurrentPublishers(t *testing.T) {
	const count = 128
	ids := make(chan string, count)
	var wait sync.WaitGroup
	for range count {
		wait.Add(1)
		go func() {
			defer wait.Done()
			ids <- serverMQTTClientID("slan-device-service-biz", "network-event", 1)
		}()
	}
	wait.Wait()
	close(ids)

	seen := make(map[string]struct{}, count)
	for id := range ids {
		if !strings.HasPrefix(id, "slan-device-service-biz-network-event-") {
			t.Fatalf("unexpected client ID %q", id)
		}
		if _, exists := seen[id]; exists {
			t.Fatalf("duplicate client ID %q", id)
		}
		seen[id] = struct{}{}
	}
}
