package ops

import (
	"sync"
	"time"
)

const (
	opsLoginFailureLimit = 5
	opsLoginWindow       = time.Minute
	opsLoginEntryCap     = 4096
)

type opsLoginFailureWindow struct {
	count     int
	expiresAt time.Time
}

type opsLoginLimiter struct {
	mu        sync.Mutex
	byIP      map[string]opsLoginFailureWindow
	byAccount map[string]opsLoginFailureWindow
}

func newOpsLoginLimiter() *opsLoginLimiter {
	return &opsLoginLimiter{
		byIP: make(map[string]opsLoginFailureWindow), byAccount: make(map[string]opsLoginFailureWindow),
	}
}

func (l *opsLoginLimiter) Allow(remoteIP, account string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return activeOpsLoginFailures(l.byIP, remoteIP, now) < opsLoginFailureLimit &&
		activeOpsLoginFailures(l.byAccount, account, now) < opsLoginFailureLimit
}

func (l *opsLoginLimiter) RecordFailure(remoteIP, account string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	recordCappedOpsLoginFailure(l.byIP, remoteIP, now)
	recordCappedOpsLoginFailure(l.byAccount, account, now)
}

func recordCappedOpsLoginFailure(items map[string]opsLoginFailureWindow, key string, now time.Time) {
	if len(items) >= opsLoginEntryCap {
		removeExpiredOpsLoginFailures(items, now)
	}
	if len(items) < opsLoginEntryCap || activeOpsLoginFailures(items, key, now) > 0 {
		recordOpsLoginFailure(items, key, now)
	}
}

func (l *opsLoginLimiter) RecordSuccess(remoteIP, account string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.byIP, remoteIP)
	delete(l.byAccount, account)
}

func activeOpsLoginFailures(items map[string]opsLoginFailureWindow, key string, now time.Time) int {
	item, found := items[key]
	if !found || !now.Before(item.expiresAt) {
		delete(items, key)
		return 0
	}
	return item.count
}

func recordOpsLoginFailure(items map[string]opsLoginFailureWindow, key string, now time.Time) {
	item := items[key]
	if !now.Before(item.expiresAt) {
		item = opsLoginFailureWindow{expiresAt: now.Add(opsLoginWindow)}
	}
	item.count++
	items[key] = item
}

func removeExpiredOpsLoginFailures(items map[string]opsLoginFailureWindow, now time.Time) {
	for key, item := range items {
		if !now.Before(item.expiresAt) {
			delete(items, key)
		}
	}
}
