package mqtt

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	serviceapi "github.com/slan/service-biz/internal/api"
)

const (
	mqttAuthIdentityFailureLimit = 10
	mqttAuthSourceFailureLimit   = 1000
	mqttAuthFailureWindow        = time.Minute
	mqttAuthLimiterEntryCap      = 8192
)

type mqttAuthFailure struct {
	count     int
	expiresAt time.Time
}

type mqttAuthLimiter struct {
	mu         sync.Mutex
	bySource   map[string]mqttAuthFailure
	byIdentity map[string]mqttAuthFailure
}

func newMQTTAuthLimiter() *mqttAuthLimiter {
	return &mqttAuthLimiter{
		bySource: make(map[string]mqttAuthFailure), byIdentity: make(map[string]mqttAuthFailure),
	}
}

func (l *mqttAuthLimiter) Allow(source, identity string, now time.Time) bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return activeMQTTAuthFailures(l.bySource, source, now) < mqttAuthSourceFailureLimit &&
		activeMQTTAuthFailures(l.byIdentity, identity, now) < mqttAuthIdentityFailureLimit
}

func (l *mqttAuthLimiter) RecordFailure(source, identity string, now time.Time) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.bySource)+len(l.byIdentity) >= mqttAuthLimiterEntryCap {
		removeExpiredMQTTAuthFailures(l.bySource, now)
		removeExpiredMQTTAuthFailures(l.byIdentity, now)
	}
	if _, exists := l.bySource[source]; exists || len(l.bySource)+len(l.byIdentity) < mqttAuthLimiterEntryCap {
		recordMQTTAuthFailure(l.bySource, source, now)
	}
	if _, exists := l.byIdentity[identity]; exists || len(l.bySource)+len(l.byIdentity) < mqttAuthLimiterEntryCap {
		recordMQTTAuthFailure(l.byIdentity, identity, now)
	}
}

func (l *mqttAuthLimiter) RecordSuccess(identity string) {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.byIdentity, identity)
}

func mqttAuthIdentity(clientID, username string) string {
	digest := sha256.Sum256([]byte(strings.TrimSpace(clientID) + "\x00" + strings.TrimSpace(username)))
	return hex.EncodeToString(digest[:])
}

func mqttAuthSource(r *http.Request) string {
	return serviceapi.RemoteIP(r)
}

func activeMQTTAuthFailures(items map[string]mqttAuthFailure, key string, now time.Time) int {
	item, ok := items[key]
	if !ok || !now.Before(item.expiresAt) {
		delete(items, key)
		return 0
	}
	return item.count
}

func recordMQTTAuthFailure(items map[string]mqttAuthFailure, key string, now time.Time) {
	item := items[key]
	if !now.Before(item.expiresAt) {
		item = mqttAuthFailure{expiresAt: now.Add(mqttAuthFailureWindow)}
	}
	item.count++
	items[key] = item
}

func removeExpiredMQTTAuthFailures(items map[string]mqttAuthFailure, now time.Time) {
	for key, item := range items {
		if !now.Before(item.expiresAt) {
			delete(items, key)
		}
	}
}
