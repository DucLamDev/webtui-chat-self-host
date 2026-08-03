package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHTTPMetricsExportsLatencyHistogram(t *testing.T) {
	gin.SetMode(gin.TestMode)
	metrics := NewHTTPMetrics()
	router := gin.New()
	router.Use(metrics.Middleware())
	router.GET("/ping", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	router.GET("/metrics", metrics.Handler())

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ping", nil))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := w.Body.String()
	if !strings.Contains(body, "webtui_http_request_latency_seconds_bucket") ||
		!strings.Contains(body, "le=\"+Inf\"") {
		t.Fatalf("metrics missing request latency histogram: %s", body)
	}
	if strings.Contains(body, `path="/metrics"`) {
		t.Fatalf("metrics scrape must not pollute API latency: %s", body)
	}
}

func TestHTTPMetricsExcludesInfrastructureProbes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	metrics := NewHTTPMetrics()
	router := gin.New()
	router.Use(metrics.Middleware())
	router.GET("/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/ready", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/metrics", metrics.Handler())

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/health", nil))
	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ready", nil))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if strings.Contains(w.Body.String(), `path="/health"`) || strings.Contains(w.Body.String(), `path="/ready"`) {
		t.Fatalf("probe traffic must not pollute API latency: %s", w.Body.String())
	}
}

func TestHTTPMetricsExcludesConfiguredScrapePath(t *testing.T) {
	gin.SetMode(gin.TestMode)
	metrics := NewHTTPMetrics("/internal/telemetry")
	router := gin.New()
	router.Use(metrics.Middleware())
	router.GET("/internal/telemetry", metrics.Handler())

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/internal/telemetry", nil))
	if strings.Contains(w.Body.String(), `path="/internal/telemetry"`) {
		t.Fatalf("custom scrape path must not pollute API latency: %s", w.Body.String())
	}
}

func TestHTTPMetricsDoesNotUseRawUnmatchedPathAsLabel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	metrics := NewHTTPMetrics()
	router := gin.New()
	router.Use(metrics.Middleware())
	router.GET("/metrics", metrics.Handler())

	router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/private/user/123", nil))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := w.Body.String()
	if !strings.Contains(body, "path=\"unmatched\"") {
		t.Fatalf("metrics missing bounded unmatched label: %s", body)
	}
	if strings.Contains(body, "/private/user/123") {
		t.Fatalf("metrics leaked raw unmatched path: %s", body)
	}
}
