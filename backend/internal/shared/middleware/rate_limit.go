package middleware

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	sharedauth "github.com/duclamdev/application-chat/backend/internal/shared/auth"
	"github.com/duclamdev/application-chat/backend/internal/shared/response"
	"github.com/gin-gonic/gin"
)

type rateLimitEntry struct {
	count   int
	resetAt time.Time
}

type rateLimitBucket struct {
	key   string
	limit int
}

// RateLimit protects the small set of unauthenticated endpoints where request
// flooding can create accounts or guess passwords. Normal application traffic
// is deliberately not counted here: chat clients perform several background
// requests and must never lock users out simply because they are active.
func RateLimit(perMinute int, burst int, _ *sharedauth.Manager) gin.HandlerFunc {
	limit := perMinute + burst
	if limit <= 0 {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	var mu sync.Mutex
	entries := make(map[string]rateLimitEntry)
	lastCleanup := time.Now()
	window := time.Minute

	return func(c *gin.Context) {
		routeKey := sensitiveAuthRateLimitRoute(c.Request.Method, c.Request.URL.Path)
		if routeKey == "" {
			c.Next()
			return
		}

		now := time.Now()
		bucket := rateLimitBucket{
			key:   "auth:" + routeKey + ":ip:" + c.ClientIP(),
			limit: limit,
		}

		mu.Lock()
		if now.Sub(lastCleanup) > window {
			for existingKey, existingEntry := range entries {
				if now.After(existingEntry.resetAt) {
					delete(entries, existingKey)
				}
			}
			lastCleanup = now
		}

		entry := entries[bucket.key]
		if entry.resetAt.IsZero() || now.After(entry.resetAt) {
			entry = rateLimitEntry{
				count:   0,
				resetAt: now.Add(window),
			}
		}

		if entry.count >= bucket.limit {
			retryAfter := int(time.Until(entry.resetAt).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			entries[bucket.key] = entry
			mu.Unlock()

			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.Header("X-RateLimit-Limit", strconv.Itoa(bucket.limit))
			c.Header("X-RateLimit-Remaining", "0")
			response.Fail(c, http.StatusTooManyRequests, "AUTH_TEMPORARILY_UNAVAILABLE", "Yêu cầu đăng nhập tạm thời chưa thể xử lý. Vui lòng thử lại sau ít phút.", nil)
			c.Abort()
			return
		}

		entry.count++
		entries[bucket.key] = entry
		remaining := bucket.limit - entry.count
		resetAt := entry.resetAt
		mu.Unlock()

		c.Header("X-RateLimit-Limit", strconv.Itoa(bucket.limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))

		c.Next()
	}
}

func sensitiveAuthRateLimitRoute(method string, path string) string {
	if method != http.MethodPost {
		return ""
	}
	switch path {
	case "/api/v1/auth/register", "/api/v1/auth/login":
		return path
	default:
		return ""
	}
}
