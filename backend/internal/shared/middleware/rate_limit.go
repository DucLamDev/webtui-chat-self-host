package middleware

import (
	"net/http"
	"strconv"
	"strings"
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

func RateLimit(perMinute int, burst int, tokens *sharedauth.Manager) gin.HandlerFunc {
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
		now := time.Now()
		buckets := rateLimitBuckets(c, limit, tokens)

		mu.Lock()
		if now.Sub(lastCleanup) > window {
			for existingKey, entry := range entries {
				if now.After(entry.resetAt) {
					delete(entries, existingKey)
				}
			}
			lastCleanup = now
		}

		current := make([]rateLimitEntry, len(buckets))
		blockedIndex := -1
		for index, bucket := range buckets {
			entry := entries[bucket.key]
			if entry.resetAt.IsZero() || now.After(entry.resetAt) {
				entry = rateLimitEntry{
					count:   0,
					resetAt: now.Add(window),
				}
			}
			current[index] = entry
			if blockedIndex == -1 && entry.count >= bucket.limit {
				blockedIndex = index
			}
		}

		if blockedIndex >= 0 {
			blockedBucket := buckets[blockedIndex]
			blockedEntry := current[blockedIndex]
			retryAfter := int(time.Until(blockedEntry.resetAt).Seconds())
			if retryAfter < 1 {
				retryAfter = 1
			}
			entries[blockedBucket.key] = blockedEntry
			mu.Unlock()

			c.Header("Retry-After", strconv.Itoa(retryAfter))
			c.Header("X-RateLimit-Limit", strconv.Itoa(blockedBucket.limit))
			c.Header("X-RateLimit-Remaining", "0")
			response.Fail(c, http.StatusTooManyRequests, "RATE_LIMITED", "Bạn thao tác quá nhanh, vui lòng thử lại sau.", nil)
			c.Abort()
			return
		}

		for index, bucket := range buckets {
			current[index].count++
			entries[bucket.key] = current[index]
		}
		primaryBucket := buckets[0]
		primaryEntry := current[0]
		remaining := primaryBucket.limit - primaryEntry.count
		resetAt := primaryEntry.resetAt
		mu.Unlock()

		c.Header("X-RateLimit-Limit", strconv.Itoa(primaryBucket.limit))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Header("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))

		c.Next()
	}
}

func rateLimitBuckets(c *gin.Context, limit int, tokens *sharedauth.Manager) []rateLimitBucket {
	ipBucket := rateLimitBucket{key: "ip:" + c.ClientIP(), limit: limit}
	if strictIPRateLimitRoute(c.Request.Method, c.Request.URL.Path) {
		return []rateLimitBucket{ipBucket}
	}

	userID := verifiedAccessTokenSubject(c.GetHeader("Authorization"), tokens)
	if userID == "" {
		return []rateLimitBucket{ipBucket}
	}

	// Signed JWT subjects give authenticated users behind one office/NAT IP an
	// independent quota. An aggregate IP cap still limits abuse by many valid
	// accounts from the same source. Never derive a bucket from an unverified
	// Bearer value: arbitrary headers must remain in the strict IP bucket.
	ipBucket.limit = limit * 10
	return []rateLimitBucket{
		{key: "user:" + userID, limit: limit},
		ipBucket,
	}
}

func verifiedAccessTokenSubject(authorization string, tokens *sharedauth.Manager) string {
	if tokens == nil {
		return ""
	}

	authorization = strings.TrimSpace(authorization)
	if !strings.HasPrefix(authorization, "Bearer ") {
		return ""
	}

	rawToken := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
	if rawToken == "" {
		return ""
	}
	claims, err := tokens.VerifyAccessToken(rawToken)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(claims.Subject)
}

func strictIPRateLimitRoute(method string, path string) bool {
	if method != http.MethodPost {
		return false
	}
	switch path {
	case "/api/v1/auth/register",
		"/api/v1/auth/login",
		"/api/v1/auth/google",
		"/api/v1/auth/refresh":
		return true
	default:
		return false
	}
}
