package bootstrap

import (
	"os"
	"strings"
	"testing"
)

func TestModerationQueueHasOperationalMetricsAndAlerts(t *testing.T) {
	apiSource := readOperationsFixture(t, "api.go")
	for _, metric := range []string{
		"webtui_moderation_reports",
		"webtui_moderation_urgent_triage_overdue",
		"webtui_moderation_normal_triage_overdue",
		"webtui_moderation_closure_overdue",
		"webtui_moderation_oldest_open_age_seconds",
	} {
		if !strings.Contains(apiSource, metric) {
			t.Fatalf("operational metrics do not expose %q", metric)
		}
	}

	rules := readOperationsFixture(t, "../../../deploy/self-hosted/observability/rules/webtui-alerts.yml")
	for _, alert := range []string{
		"WebTuiModerationUrgentTriageOverdue",
		"WebTuiModerationNormalTriageOverdue",
		"WebTuiModerationClosureOverdue",
	} {
		if !strings.Contains(rules, alert) {
			t.Fatalf("Prometheus rules do not contain %q", alert)
		}
	}

	dashboard := readOperationsFixture(t, "../../../deploy/self-hosted/observability/grafana/dashboards/webtui-operations.json")
	for _, panel := range []string{"Moderation queue by status", "Oldest open moderation report", "Moderation SLO breaches"} {
		if !strings.Contains(dashboard, panel) {
			t.Fatalf("operations dashboard does not contain %q", panel)
		}
	}
}

func TestModerationEvidenceRetentionRunsInWorker(t *testing.T) {
	workerSource := readOperationsFixture(t, "worker.go")
	for _, required := range []string{
		"moderation_evidence_retention",
		"EvidenceRetentionDays",
		"target_snapshot = '{}'::jsonb",
		"details = NULL",
		"FOR UPDATE SKIP LOCKED",
	} {
		if !strings.Contains(workerSource, required) {
			t.Fatalf("moderation retention worker is missing %q", required)
		}
	}
}

func readOperationsFixture(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(contents)
}
