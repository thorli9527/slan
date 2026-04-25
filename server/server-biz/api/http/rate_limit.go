package httpapi

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
)

const authRateLimitWindow = time.Minute

type requestRateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]rateLimitEntry
}

type rateLimitEntry struct {
	windowStart time.Time
	count       int
}

func newRequestRateLimiter(limit int, window time.Duration) *requestRateLimiter {
	return &requestRateLimiter{
		limit:   limit,
		window:  window,
		entries: make(map[string]rateLimitEntry),
	}
}

func (l *requestRateLimiter) allow(key string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	for existingKey, entry := range l.entries {
		if now.Sub(entry.windowStart) >= l.window {
			delete(l.entries, existingKey)
		}
	}

	entry := l.entries[key]
	if entry.windowStart.IsZero() || now.Sub(entry.windowStart) >= l.window {
		l.entries[key] = rateLimitEntry{windowStart: now, count: 1}
		return true
	}
	if entry.count >= l.limit {
		return false
	}
	entry.count++
	l.entries[key] = entry
	return true
}

func rateLimitByIP(limit int, window time.Duration) gin.HandlerFunc {
	limiter := newRequestRateLimiter(limit, window)
	return func(c *gin.Context) {
		key := c.ClientIP() + ":" + c.FullPath()
		if !limiter.allow(key, time.Now()) {
			c.JSON(http.StatusTooManyRequests, dto.ErrorResponse{
				Code:    "RATE_LIMITED",
				Message: "too many requests, please retry later",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}
