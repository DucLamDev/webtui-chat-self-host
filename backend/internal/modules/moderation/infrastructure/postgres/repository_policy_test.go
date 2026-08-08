package postgres

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestCreateReportSerializesQuotaCheckAndInsert(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() did not return the test filename")
	}
	contentBytes, err := os.ReadFile(strings.TrimSuffix(filename, "repository_policy_test.go") + "repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	content := string(contentBytes)
	start := strings.Index(content, "func (r *Repository) CreateReport")
	if start < 0 {
		t.Fatal("CreateReport repository implementation not found")
	}
	end := strings.Index(content[start:], "func (r *Repository) ListReports")
	if end < 0 {
		t.Fatal("CreateReport repository implementation has no boundary")
	}
	implementation := content[start : start+end]
	lockIndex := strings.Index(implementation, "pg_advisory_xact_lock")
	countIndex := strings.Index(implementation, "SELECT count(*)")
	insertIndex := strings.Index(implementation, "INSERT INTO moderation_reports")
	commitIndex := strings.Index(implementation, "tx.Commit")
	if lockIndex < 0 || countIndex < lockIndex || insertIndex < countIndex || commitIndex < insertIndex {
		t.Fatalf("quota lock/count/insert/commit must remain ordered in one transaction")
	}
	if !strings.Contains(implementation, "moderationdomain.ErrReportRateLimit") {
		t.Fatal("atomic report quota must fail closed at the repository boundary")
	}
	if !strings.Contains(implementation, "RETURNING id::text, created_at, updated_at") ||
		strings.Contains(implementation, "r.findReport") {
		t.Fatal("CreateReport must build its receipt from INSERT RETURNING and never fail on a post-commit read")
	}
}

func TestReportTargetSnapshotExcludesAttachmentsAndMetadata(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	contentBytes, err := os.ReadFile(strings.TrimSuffix(filename, "repository_policy_test.go") + "repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	content := string(contentBytes)
	for _, required := range []string{"body_excerpt", "body_sha256", "left(message.body, 2000)", "target_snapshot"} {
		if !strings.Contains(content, required) {
			t.Fatalf("immutable report evidence is missing %q", required)
		}
	}
	snapshotStart := strings.Index(content, "'message_id', message.id::text")
	if snapshotStart < 0 {
		t.Fatal("message snapshot SQL not found")
	}
	snapshotEnd := strings.Index(content[snapshotStart:], ")::text")
	if snapshotEnd < 0 {
		t.Fatal("message snapshot SQL has no boundary")
	}
	snapshot := content[snapshotStart : snapshotStart+snapshotEnd]
	if strings.Contains(snapshot, "message.metadata") || strings.Contains(snapshot, "attachment") {
		t.Fatal("moderation snapshot must not copy message metadata or attachment payloads")
	}
}

func TestReportTargetSupportsUserEventsAndSenderlessProducers(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	contentBytes, err := os.ReadFile(strings.TrimSuffix(filename, "repository_policy_test.go") + "repository.go")
	if err != nil {
		t.Fatalf("read repository.go: %v", err)
	}
	content := string(contentBytes)
	for _, required := range []string{
		"message.kind IN ('text', 'file', 'bot', 'event')",
		"LEFT JOIN users sender",
		"COALESCE(message.sender_id::text, '')",
		"'producer_kind'",
		"NULLIF($5, '')::uuid",
		"if strings.TrimSpace(params.TargetUserID) != \"\"",
	} {
		if !strings.Contains(content, required) {
			t.Fatalf("reportable event/senderless target support is missing %q", required)
		}
	}
}
