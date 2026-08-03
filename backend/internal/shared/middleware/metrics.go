package middleware

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type HTTPMetrics struct {
	mu         sync.RWMutex
	started    time.Time
	requests   map[metricLabels]metricValue
	gaugeFuncs []GaugeFunc
	excluded   map[string]struct{}
}

type GaugeFunc func() []Gauge

type Gauge struct {
	Name   string
	Help   string
	Labels map[string]string
	Value  float64
}

type metricLabels struct {
	Method string
	Path   string
	Status string
}

type metricValue struct {
	Count           int64
	DurationSeconds float64
	DurationBuckets []int64
}

var httpDurationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30}

func NewHTTPMetrics(excludedPaths ...string) *HTTPMetrics {
	metrics := &HTTPMetrics{
		started:  time.Now(),
		requests: make(map[metricLabels]metricValue),
		excluded: map[string]struct{}{`/metrics`: {}, `/health`: {}, `/ready`: {}},
	}
	for _, path := range excludedPaths {
		if path = strings.TrimSpace(path); path != "" {
			metrics.excluded[path] = struct{}{}
		}
	}
	return metrics
}

func (m *HTTPMetrics) RegisterGaugeFunc(fn GaugeFunc) {
	if fn == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.gaugeFuncs = append(m.gaugeFuncs, fn)
}

func (m *HTTPMetrics) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			// Avoid leaking identifiers and creating unbounded metric labels for
			// unmatched URLs.
			path = "unmatched"
		}
		if _, excluded := m.excluded[path]; excluded {
			// Scrapes and liveness/readiness probes are infrastructure traffic.
			// Including them makes p95/p99 misleading on low-traffic self-hosts and
			// lets /metrics measure the DB-backed metric queries it runs itself.
			return
		}

		labels := metricLabels{
			Method: c.Request.Method,
			Path:   path,
			Status: strconv.Itoa(c.Writer.Status()),
		}

		m.mu.Lock()
		value := m.requests[labels]
		duration := time.Since(start).Seconds()
		value.Count++
		value.DurationSeconds += duration
		if value.DurationBuckets == nil {
			value.DurationBuckets = make([]int64, len(httpDurationBuckets))
		}
		for index, upperBound := range httpDurationBuckets {
			if duration <= upperBound {
				value.DurationBuckets[index]++
			}
		}
		m.requests[labels] = value
		m.mu.Unlock()
	}
}

func (m *HTTPMetrics) Handler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		c.Status(http.StatusOK)

		snapshot := m.snapshot()
		uptime := time.Since(m.started).Seconds()

		fmt.Fprintln(c.Writer, "# HELP webtui_process_uptime_seconds Thời gian process đã chạy theo giây.")
		fmt.Fprintln(c.Writer, "# TYPE webtui_process_uptime_seconds gauge")
		fmt.Fprintf(c.Writer, "webtui_process_uptime_seconds %.3f\n", uptime)
		fmt.Fprintln(c.Writer, "# HELP webtui_http_requests_total Tổng số HTTP request đã xử lý.")
		fmt.Fprintln(c.Writer, "# TYPE webtui_http_requests_total counter")
		for _, item := range snapshot {
			fmt.Fprintf(c.Writer,
				"webtui_http_requests_total{method=\"%s\",path=\"%s\",status=\"%s\"} %d\n",
				escapeLabel(item.labels.Method),
				escapeLabel(item.labels.Path),
				escapeLabel(item.labels.Status),
				item.value.Count,
			)
		}
		fmt.Fprintln(c.Writer, "# HELP webtui_http_request_duration_seconds_sum Tổng thời gian xử lý HTTP request theo giây.")
		fmt.Fprintln(c.Writer, "# TYPE webtui_http_request_duration_seconds_sum counter")
		for _, item := range snapshot {
			fmt.Fprintf(c.Writer,
				"webtui_http_request_duration_seconds_sum{method=\"%s\",path=\"%s\",status=\"%s\"} %.6f\n",
				escapeLabel(item.labels.Method),
				escapeLabel(item.labels.Path),
				escapeLabel(item.labels.Status),
				item.value.DurationSeconds,
			)
		}

		fmt.Fprintln(c.Writer, "# HELP webtui_http_request_latency_seconds HTTP request latency distribution in seconds.")
		fmt.Fprintln(c.Writer, "# TYPE webtui_http_request_latency_seconds histogram")
		for _, item := range snapshot {
			for index, upperBound := range httpDurationBuckets {
				fmt.Fprintf(c.Writer,
					"webtui_http_request_latency_seconds_bucket{method=\"%s\",path=\"%s\",status=\"%s\",le=\"%s\"} %d\n",
					escapeLabel(item.labels.Method),
					escapeLabel(item.labels.Path),
					escapeLabel(item.labels.Status),
					strconv.FormatFloat(upperBound, 'f', -1, 64),
					item.value.DurationBuckets[index],
				)
			}
			fmt.Fprintf(c.Writer,
				"webtui_http_request_latency_seconds_bucket{method=\"%s\",path=\"%s\",status=\"%s\",le=\"+Inf\"} %d\n",
				escapeLabel(item.labels.Method),
				escapeLabel(item.labels.Path),
				escapeLabel(item.labels.Status),
				item.value.Count,
			)
			fmt.Fprintf(c.Writer,
				"webtui_http_request_latency_seconds_sum{method=\"%s\",path=\"%s\",status=\"%s\"} %.6f\n",
				escapeLabel(item.labels.Method),
				escapeLabel(item.labels.Path),
				escapeLabel(item.labels.Status),
				item.value.DurationSeconds,
			)
			fmt.Fprintf(c.Writer,
				"webtui_http_request_latency_seconds_count{method=\"%s\",path=\"%s\",status=\"%s\"} %d\n",
				escapeLabel(item.labels.Method),
				escapeLabel(item.labels.Path),
				escapeLabel(item.labels.Status),
				item.value.Count,
			)
		}

		seenGaugeFamilies := make(map[string]struct{})
		for _, gauge := range m.gauges() {
			_, familySeen := seenGaugeFamilies[gauge.Name]
			if !familySeen && gauge.Help != "" {
				fmt.Fprintf(c.Writer, "# HELP %s %s\n", gauge.Name, escapeHelp(gauge.Help))
			}
			if !familySeen {
				fmt.Fprintf(c.Writer, "# TYPE %s gauge\n", gauge.Name)
				seenGaugeFamilies[gauge.Name] = struct{}{}
			}
			fmt.Fprintf(c.Writer, "%s%s %.6f\n", gauge.Name, formatLabels(gauge.Labels), gauge.Value)
		}
	}
}

type metricSnapshotItem struct {
	labels metricLabels
	value  metricValue
}

func (m *HTTPMetrics) snapshot() []metricSnapshotItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]metricSnapshotItem, 0, len(m.requests))
	for labels, value := range m.requests {
		value.DurationBuckets = append([]int64(nil), value.DurationBuckets...)
		items = append(items, metricSnapshotItem{labels: labels, value: value})
	}

	sort.Slice(items, func(i, j int) bool {
		left := items[i].labels.Method + items[i].labels.Path + items[i].labels.Status
		right := items[j].labels.Method + items[j].labels.Path + items[j].labels.Status
		return left < right
	})
	return items
}

func (m *HTTPMetrics) gauges() []Gauge {
	m.mu.RLock()
	funcs := append([]GaugeFunc(nil), m.gaugeFuncs...)
	m.mu.RUnlock()

	gauges := make([]Gauge, 0)
	for _, fn := range funcs {
		gauges = append(gauges, fn()...)
	}
	sort.Slice(gauges, func(i, j int) bool {
		return gauges[i].Name < gauges[j].Name
	})
	return gauges
}

func formatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=\"%s\"", key, escapeLabel(labels[key])))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func escapeLabel(value string) string {
	value = strings.ReplaceAll(value, "\\", "\\\\")
	value = strings.ReplaceAll(value, "\n", "\\n")
	value = strings.ReplaceAll(value, "\"", "\\\"")
	return value
}

func escapeHelp(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\\", "\\\\")
	return value
}
