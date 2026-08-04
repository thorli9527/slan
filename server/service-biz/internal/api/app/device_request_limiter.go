package app

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
	deviceSessionRenewLimit    = 30
	deviceRuntimeReportLimit   = 120
	deviceSessionRenewIPLimit  = 3000
	deviceRuntimeReportIPLimit = 12000
	deviceRequestLimitWindow   = time.Minute
	deviceRequestEntryCap      = 8192
)

type deviceRequestWindow struct {
	count     int
	expiresAt time.Time
}

type deviceRequestLimiter struct {
	mu            sync.Mutex
	identityLimit int
	ipLimit       int
	byIP          map[string]deviceRequestWindow
	byIdentity    map[string]deviceRequestWindow
}

func newDeviceRequestLimiter(identityLimit, ipLimit int) *deviceRequestLimiter {
	return &deviceRequestLimiter{
		identityLimit: identityLimit, ipLimit: ipLimit,
		byIP: make(map[string]deviceRequestWindow), byIdentity: make(map[string]deviceRequestWindow),
	}
}

func (l *deviceRequestLimiter) Allow(remoteIP, identity string, now time.Time) bool {
	if l == nil || l.identityLimit <= 0 || l.ipLimit <= 0 {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	remoteIP = strings.TrimSpace(remoteIP)
	identity = strings.TrimSpace(identity)
	if len(l.byIP)+len(l.byIdentity) >= deviceRequestEntryCap {
		removeExpiredDeviceRequests(l.byIP, now)
		removeExpiredDeviceRequests(l.byIdentity, now)
		_, knownIP := l.byIP[remoteIP]
		_, knownIdentity := l.byIdentity[identity]
		newEntries := 0
		if !knownIP {
			newEntries++
		}
		if identity != "" && !knownIdentity {
			newEntries++
		}
		if len(l.byIP)+len(l.byIdentity)+newEntries > deviceRequestEntryCap {
			return false
		}
	}
	if activeDeviceRequestCount(l.byIP, remoteIP, now) >= l.ipLimit ||
		(identity != "" && activeDeviceRequestCount(l.byIdentity, identity, now) >= l.identityLimit) {
		return false
	}
	recordDeviceRequest(l.byIP, remoteIP, now)
	if identity != "" {
		recordDeviceRequest(l.byIdentity, identity, now)
	}
	return true
}

func deviceRequestIdentity(values ...string) string {
	hash := sha256.New()
	written := false
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		_, _ = hash.Write([]byte(value))
		_, _ = hash.Write([]byte{0})
		written = true
	}
	if !written {
		return ""
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func deviceRequestRemoteIP(r *http.Request) string {
	return serviceapi.RemoteIP(r)
}

func activeDeviceRequestCount(items map[string]deviceRequestWindow, key string, now time.Time) int {
	item, ok := items[key]
	if !ok || !now.Before(item.expiresAt) {
		delete(items, key)
		return 0
	}
	return item.count
}

func recordDeviceRequest(items map[string]deviceRequestWindow, key string, now time.Time) {
	item := items[key]
	if !now.Before(item.expiresAt) {
		item = deviceRequestWindow{expiresAt: now.Add(deviceRequestLimitWindow)}
	}
	item.count++
	items[key] = item
}

func removeExpiredDeviceRequests(items map[string]deviceRequestWindow, now time.Time) {
	for key, item := range items {
		if !now.Before(item.expiresAt) {
			delete(items, key)
		}
	}
}
