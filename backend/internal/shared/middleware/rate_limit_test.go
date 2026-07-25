package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sharedauth "github.com/duclamdev/application-chat/backend/internal/shared/auth"
	"github.com/gin-gonic/gin"
)

func newRateLimitTestTokenManager() *sharedauth.Manager {
	return sharedauth.NewManager(
		"rate-limit-test-access-secret-that-is-long-enough",
		"rate-limit-test-refresh-secret-that-is-long-enough",
		15*time.Minute,
		24*time.Hour,
	)
}

func createRateLimitTestAccessToken(t *testing.T, tokens *sharedauth.Manager, userID string) string {
	t.Helper()
	token, _, err := tokens.CreateAccessToken(userID, userID+"@example.com", userID)
	if err != nil {
		t.Fatalf("CreateAccessToken(%q): %v", userID, err)
	}
	return token
}

func TestRateLimitBlocksAfterLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tokens := newRateLimitTestTokenManager()
	router := gin.New()
	router.Use(RateLimit(1, 0, tokens))
	router.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	first := httptest.NewRecorder()
	firstReq := httptest.NewRequest(http.MethodGet, "/ping", nil)
	firstReq.RemoteAddr = "127.0.0.1:1000"
	router.ServeHTTP(first, firstReq)
	if first.Code != http.StatusNoContent {
		t.Fatalf("request đầu status = %d, muốn %d", first.Code, http.StatusNoContent)
	}

	second := httptest.NewRecorder()
	secondReq := httptest.NewRequest(http.MethodGet, "/ping", nil)
	secondReq.RemoteAddr = "127.0.0.1:1001"
	router.ServeHTTP(second, secondReq)
	if second.Code != http.StatusTooManyRequests {
		t.Fatalf("request thứ hai status = %d, muốn %d", second.Code, http.StatusTooManyRequests)
	}
	if got := second.Header().Get("Retry-After"); got == "" {
		t.Fatal("Retry-After không được để trống")
	}
}

func TestRateLimitSeparatesAuthenticatedUsersBehindSameIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tokens := newRateLimitTestTokenManager()
	router := gin.New()
	router.Use(RateLimit(1, 0, tokens))
	router.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := func(token string) *httptest.ResponseRecorder {
		writer := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = "127.0.0.1:1000"
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(writer, req)
		return writer
	}

	for _, userID := range []string{
		"user-one",
		"user-two",
		"user-three",
		"user-four",
		"user-five",
		"user-six",
	} {
		token := createRateLimitTestAccessToken(t, tokens, userID)
		if status := request(token).Code; status != http.StatusNoContent {
			t.Fatalf("first request for %s status = %d, want %d", userID, status, http.StatusNoContent)
		}
	}
	if status := request(createRateLimitTestAccessToken(t, tokens, "user-one")).Code; status != http.StatusTooManyRequests {
		t.Fatalf("user one repeated status = %d, want %d", status, http.StatusTooManyRequests)
	}
}

func TestRateLimitDoesNotTrustArbitraryBearerTokens(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tokens := newRateLimitTestTokenManager()
	router := gin.New()
	router.Use(RateLimit(1, 0, tokens))
	router.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := func(rawToken string) int {
		writer := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = "127.0.0.1:1000"
		req.Header.Set("Authorization", "Bearer "+rawToken)
		router.ServeHTTP(writer, req)
		return writer.Code
	}

	if status := request("forged-token-one"); status != http.StatusNoContent {
		t.Fatalf("first forged token status = %d, want %d", status, http.StatusNoContent)
	}
	if status := request("forged-token-two"); status != http.StatusTooManyRequests {
		t.Fatalf("second forged token status = %d, want %d", status, http.StatusTooManyRequests)
	}
}

func TestRateLimitUsesVerifiedSubjectInsteadOfTokenValue(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tokens := newRateLimitTestTokenManager()
	router := gin.New()
	router.Use(RateLimit(1, 0, tokens))
	router.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	firstToken, _, err := tokens.CreateAccessToken("same-user", "first@example.com", "first")
	if err != nil {
		t.Fatalf("create first token: %v", err)
	}
	secondToken, _, err := tokens.CreateAccessToken("same-user", "second@example.com", "second")
	if err != nil {
		t.Fatalf("create second token: %v", err)
	}

	request := func(token string) int {
		writer := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = "127.0.0.1:1000"
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(writer, req)
		return writer.Code
	}

	if status := request(firstToken); status != http.StatusNoContent {
		t.Fatalf("first token status = %d, want %d", status, http.StatusNoContent)
	}
	if status := request(secondToken); status != http.StatusTooManyRequests {
		t.Fatalf("second token for same subject status = %d, want %d", status, http.StatusTooManyRequests)
	}
}

func TestRateLimitKeepsAggregateIPCapForVerifiedUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tokens := newRateLimitTestTokenManager()
	router := gin.New()
	router.Use(RateLimit(1, 0, tokens))
	router.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	for index := 0; index < 11; index++ {
		writer := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		req.RemoteAddr = "127.0.0.1:1000"
		token := createRateLimitTestAccessToken(t, tokens, "user-"+string(rune('a'+index)))
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(writer, req)

		wantStatus := http.StatusNoContent
		if index == 10 {
			wantStatus = http.StatusTooManyRequests
		}
		if writer.Code != wantStatus {
			t.Fatalf("request %d status = %d, want %d", index+1, writer.Code, wantStatus)
		}
	}
}

func TestRateLimitKeepsPublicAuthenticationRoutesStrictlyPerIP(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tokens := newRateLimitTestTokenManager()
	router := gin.New()
	router.Use(RateLimit(1, 0, tokens))
	router.POST("/api/v1/auth/login", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	request := func(userID string) int {
		writer := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "127.0.0.1:1000"
		token := createRateLimitTestAccessToken(t, tokens, userID)
		req.Header.Set("Authorization", "Bearer "+token)
		router.ServeHTTP(writer, req)
		return writer.Code
	}

	if status := request("user-one"); status != http.StatusNoContent {
		t.Fatalf("first login status = %d, want %d", status, http.StatusNoContent)
	}
	if status := request("user-two"); status != http.StatusTooManyRequests {
		t.Fatalf("second login status = %d, want %d", status, http.StatusTooManyRequests)
	}
}
