package relayserver

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

const relayMetricsTimeout = 2 * time.Second

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)

	ctx, cancel := context.WithTimeout(r.Context(), relayMetricsTimeout)
	defer cancel()
	stats, err := s.store.Stats(ctx, time.Now().UTC())
	if err != nil {
		writeRelayMetric(w,
			"webtui_push_relay_metrics_collection_success",
			"Whether the relay queue aggregate was collected successfully.",
			nil,
			0,
		)
		return
	}

	writeRelayMetric(w,
		"webtui_push_relay_metrics_collection_success",
		"Whether the relay queue aggregate was collected successfully.",
		nil,
		1,
	)
	writeRelayMetricHeader(w,
		"webtui_push_relay_jobs",
		"Current official push relay jobs by status.",
	)
	for _, item := range []struct {
		status string
		value  int64
	}{
		{status: "pending", value: stats.Pending},
		{status: "processing", value: stats.Processing},
		{status: "retry", value: stats.Retry},
		{status: "sent", value: stats.Sent},
		{status: "dead", value: stats.Dead},
	} {
		fmt.Fprintf(w, "webtui_push_relay_jobs{status=\"%s\"} %.6f\n", item.status, float64(item.value))
	}
	writeRelayMetric(w,
		"webtui_push_relay_sent_jobs_24h",
		"Official push relay jobs delivered during the last 24 hours.",
		nil,
		float64(stats.Sent24Hours),
	)
	writeRelayMetric(w,
		"webtui_push_relay_dead_jobs_24h",
		"Official push relay jobs moved to dead-letter during the last 24 hours.",
		nil,
		float64(stats.Dead24Hours),
	)
	writeRelayMetric(w,
		"webtui_push_relay_oldest_queued_age_seconds",
		"Age of the oldest pending, processing, or retry official relay job.",
		nil,
		stats.OldestQueuedAgeSeconds,
	)
	terminal := stats.Sent24Hours + stats.Dead24Hours
	deliveryRate := 1.0
	if terminal > 0 {
		deliveryRate = float64(stats.Sent24Hours) / float64(terminal)
	}
	writeRelayMetric(w,
		"webtui_push_relay_delivery_rate_ratio_24h",
		"Ratio of sent jobs among terminal official relay jobs during the last 24 hours; one when there is no terminal traffic.",
		nil,
		deliveryRate,
	)
}

func writeRelayMetric(w http.ResponseWriter, name string, help string, labels map[string]string, value float64) {
	writeRelayMetricHeader(w, name, help)
	if len(labels) == 0 {
		fmt.Fprintf(w, "%s %.6f\n", name, value)
		return
	}
	// The relay exposes only the fixed status label values above. Keeping this
	// formatter private prevents publisher IDs or request-controlled data from
	// becoming metric labels later by accident.
	fmt.Fprintf(w, "%s{status=\"%s\"} %.6f\n", name, labels["status"], value)
}

func writeRelayMetricHeader(w http.ResponseWriter, name string, help string) {
	fmt.Fprintf(w, "# HELP %s %s\n", name, help)
	fmt.Fprintf(w, "# TYPE %s gauge\n", name)
}
