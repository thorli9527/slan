package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/slan/server/server-biz/api/dto"
)

const (
	defaultOpsLoginRateLimitWindow  = time.Minute
	defaultOpsLoginIPLimit          = 10
	defaultOpsLoginIPLoginNameLimit = 5
	defaultOpsLoginLoginNameLimit   = 20
	rateLimitJSONFieldContextPrefix = "rateLimitJSONField:"
)

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

type rateLimitDecision struct {
	allowed    bool
	retryAfter time.Duration
}

func newRequestRateLimiter(limit int, window time.Duration) *requestRateLimiter {
	return &requestRateLimiter{
		limit:   limit,
		window:  window,
		entries: make(map[string]rateLimitEntry),
	}
}

func (l *requestRateLimiter) allow(key string, now time.Time) bool {
	return l.check(key, now).allowed
}

func (l *requestRateLimiter) check(key string, now time.Time) rateLimitDecision {
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
		return rateLimitDecision{allowed: true}
	}
	if entry.count >= l.limit {
		retryAfter := l.window - now.Sub(entry.windowStart)
		if retryAfter < 0 {
			retryAfter = 0
		}
		return rateLimitDecision{retryAfter: retryAfter}
	}
	entry.count++
	l.entries[key] = entry
	return rateLimitDecision{allowed: true}
}

func rateLimitByIP(limit int, window time.Duration) gin.HandlerFunc {
	limiter := newRequestRateLimiter(limit, window)
	return func(c *gin.Context) {
		key := c.ClientIP() + ":" + c.FullPath()
		decision := limiter.check(key, time.Now())
		if !decision.allowed {
			abortRateLimited(c, decision.retryAfter)
			return
		}
		c.Next()
	}
}

func rateLimitByIPAndJSONField(limit int, window time.Duration, field string) gin.HandlerFunc {
	limiter := newRequestRateLimiter(limit, window)
	return func(c *gin.Context) {
		fieldValue, ok := requestJSONField(c, field)
		if !ok {
			abortRequestTooLarge(c)
			return
		}
		key := c.ClientIP() + ":" + c.FullPath()
		if fieldValue != "" {
			key += ":" + rateLimitJSONFieldKey(field, fieldValue)
		}
		decision := limiter.check(key, time.Now())
		if !decision.allowed {
			abortRateLimited(c, decision.retryAfter)
			return
		}
		c.Next()
	}
}

func rateLimitByJSONField(limit int, window time.Duration, field string) gin.HandlerFunc {
	limiter := newRequestRateLimiter(limit, window)
	return func(c *gin.Context) {
		fieldValue, ok := requestJSONField(c, field)
		if !ok {
			abortRequestTooLarge(c)
			return
		}
		if fieldValue == "" {
			c.Next()
			return
		}
		key := c.FullPath() + ":" + rateLimitJSONFieldKey(field, fieldValue)
		decision := limiter.check(key, time.Now())
		if !decision.allowed {
			abortRateLimited(c, decision.retryAfter)
			return
		}
		c.Next()
	}
}

func abortRateLimited(c *gin.Context, retryAfter time.Duration) {
	if retryAfter > 0 {
		c.Header("Retry-After", strconv.Itoa(ceilSeconds(retryAfter)))
	}
	c.JSON(http.StatusTooManyRequests, dto.ErrorResponse{
		Code:    "RATE_LIMITED",
		Message: "too many requests, please retry later",
	})
	c.Abort()
}

func ceilSeconds(value time.Duration) int {
	if value <= 0 {
		return 0
	}
	return int((value + time.Second - time.Nanosecond) / time.Second)
}

func rateLimitJSONFieldKey(field, value string) string {
	sum := sha256.Sum256([]byte(value))
	return field + "=" + hex.EncodeToString(sum[:])
}

func abortRequestTooLarge(c *gin.Context) {
	c.JSON(http.StatusRequestEntityTooLarge, dto.ErrorResponse{
		Code:    "REQUEST_TOO_LARGE",
		Message: "request body exceeds 1 MiB limit",
	})
	c.Abort()
}

func requestJSONField(c *gin.Context, field string) (string, bool) {
	contextKey := rateLimitJSONFieldContextPrefix + field
	if value, exists := c.Get(contextKey); exists {
		fieldValue, _ := value.(string)
		return fieldValue, true
	}
	if c.Request == nil || c.Request.Body == nil {
		return "", true
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxHTTPJSONBodyBytes+1))
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	if err != nil {
		return "", false
	}
	if int64(len(body)) > maxHTTPJSONBodyBytes {
		return "", false
	}
	if len(body) == 0 {
		return "", err == nil
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", true
	}
	value, _ := payload[field].(string)
	fieldValue := strings.TrimSpace(strings.ToLower(value))
	c.Set(contextKey, fieldValue)
	return fieldValue, true
}
