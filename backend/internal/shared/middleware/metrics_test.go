package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHTTPMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)

	metrics := NewHTTPMetrics()
	metrics.RegisterGaugeFunc(func() []Gauge {
		return []Gauge{
			{
				Name:   "webtui_test_gauge",
				Help:   "Metric kiểm thử.",
				Labels: map[string]string{"kind": "unit"},
				Value:  1,
			},
		}
	})
	router := gin.New()
	router.Use(metrics.Middleware())
	router.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	router.GET("/metrics", metrics.Handler())

	ping := httptest.NewRecorder()
	router.ServeHTTP(ping, httptest.NewRequest(http.MethodGet, "/ping", nil))
	if ping.Code != http.StatusNoContent {
		t.Fatalf("status /ping = %d, muốn %d", ping.Code, http.StatusNoContent)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := w.Body.String()
	if !strings.Contains(body, "webtui_http_requests_total") {
		t.Fatalf("metrics thiếu webtui_http_requests_total: %s", body)
	}
	if !strings.Contains(body, "path=\"/ping\"") {
		t.Fatalf("metrics thiếu route /ping: %s", body)
	}
	if !strings.Contains(body, "webtui_test_gauge{kind=\"unit\"} 1.000000") {
		t.Fatalf("metrics thiếu gauge kiểm thử: %s", body)
	}
}
