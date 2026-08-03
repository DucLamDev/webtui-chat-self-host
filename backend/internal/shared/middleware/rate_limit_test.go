package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRateLimitNeverCountsNormalApplicationTraffic(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(1, 0, nil))
	router.GET("/api/v1/channels", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	for index := 0; index < 20; index++ {
		writer := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, "/api/v1/channels", nil)
		request.RemoteAddr = "127.0.0.1:1000"
		router.ServeHTTP(writer, request)
		if writer.Code != http.StatusNoContent {
			t.Fatalf("normal request %d status = %d, want %d", index+1, writer.Code, http.StatusNoContent)
		}
		if got := writer.Header().Get("X-RateLimit-Limit"); got != "" {
			t.Fatalf("normal request unexpectedly received a rate limit header: %q", got)
		}
	}
}

func TestRateLimitProtectsLoginWithoutSharingItsQuotaWithRegistration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(1, 0, nil))
	router.POST("/api/v1/auth/login", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.POST("/api/v1/auth/register", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := func(path string) *httptest.ResponseRecorder {
		writer := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, path, nil)
		req.RemoteAddr = "127.0.0.1:1000"
		router.ServeHTTP(writer, req)
		return writer
	}

	if status := request("/api/v1/auth/login").Code; status != http.StatusNoContent {
		t.Fatalf("first login status = %d, want %d", status, http.StatusNoContent)
	}
	if status := request("/api/v1/auth/register").Code; status != http.StatusNoContent {
		t.Fatalf("first registration status = %d, want %d", status, http.StatusNoContent)
	}
	blocked := request("/api/v1/auth/login")
	if blocked.Code != http.StatusTooManyRequests {
		t.Fatalf("second login status = %d, want %d", blocked.Code, http.StatusTooManyRequests)
	}
	if got := blocked.Header().Get("Retry-After"); got == "" {
		t.Fatal("Retry-After must be present on a blocked authentication request")
	}
}

func TestRateLimitDoesNotBlockSessionRefreshOrProviderLogin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(RateLimit(1, 0, nil))
	for _, path := range []string{"/api/v1/auth/refresh", "/api/v1/auth/google", "/api/v1/auth/oidc/complete"} {
		router.POST(path, func(c *gin.Context) {
			c.Status(http.StatusNoContent)
		})
	}

	for _, path := range []string{"/api/v1/auth/refresh", "/api/v1/auth/google", "/api/v1/auth/oidc/complete"} {
		for index := 0; index < 3; index++ {
			writer := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, path, nil)
			request.RemoteAddr = "127.0.0.1:1000"
			router.ServeHTTP(writer, request)
			if writer.Code != http.StatusNoContent {
				t.Fatalf("%s request %d status = %d, want %d", path, index+1, writer.Code, http.StatusNoContent)
			}
		}
	}
}
