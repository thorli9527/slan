package app

import (
	"sync"
	"time"
)

const (
	deviceCredentialFailureLimit = 5
	deviceCredentialLimitWindow  = time.Minute
	deviceCredentialKeyEntryCap  = 4096
)

type deviceCredentialFailureWindow struct {
	count     int
	expiresAt time.Time
}

type deviceCredentialExchangeLimiter struct {
	mu    sync.Mutex
	byIP  map[string]deviceCredentialFailureWindow
	byKey map[string]deviceCredentialFailureWindow
}

func newDeviceCredentialExchangeLimiter() *deviceCredentialExchangeLimiter {
	return &deviceCredentialExchangeLimiter{
		byIP:  make(map[string]deviceCredentialFailureWindow),
		byKey: make(map[string]deviceCredentialFailureWindow),
	}
}

func (l *deviceCredentialExchangeLimiter) Allow(remoteIP, key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return activeFailureCount(l.byIP, remoteIP, now) < deviceCredentialFailureLimit &&
		activeFailureCount(l.byKey, key, now) < deviceCredentialFailureLimit
}

func (l *deviceCredentialExchangeLimiter) RecordFailure(remoteIP, key string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	recordCredentialFailure(l.byIP, remoteIP, now)
	if len(l.byKey) >= deviceCredentialKeyEntryCap {
		removeExpiredCredentialFailures(l.byKey, now)
	}
	if len(l.byKey) < deviceCredentialKeyEntryCap || activeFailureCount(l.byKey, key, now) > 0 {
		recordCredentialFailure(l.byKey, key, now)
	}
}

func (l *deviceCredentialExchangeLimiter) RecordSuccess(remoteIP, key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.byIP, remoteIP)
	delete(l.byKey, key)
}

func activeFailureCount(items map[string]deviceCredentialFailureWindow, key string, now time.Time) int {
	item, ok := items[key]
	if !ok || !now.Before(item.expiresAt) {
		delete(items, key)
		return 0
	}
	return item.count
}

func recordCredentialFailure(items map[string]deviceCredentialFailureWindow, key string, now time.Time) {
	item := items[key]
	if !now.Before(item.expiresAt) {
		item = deviceCredentialFailureWindow{expiresAt: now.Add(deviceCredentialLimitWindow)}
	}
	item.count++
	items[key] = item
}

func removeExpiredCredentialFailures(items map[string]deviceCredentialFailureWindow, now time.Time) {
	for key, item := range items {
		if !now.Before(item.expiresAt) {
			delete(items, key)
		}
	}
}
