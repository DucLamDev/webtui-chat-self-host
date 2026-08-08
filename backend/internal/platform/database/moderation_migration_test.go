package database

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestUGCModerationMigrationContainsSafetyAndLegalControls(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() did not return the test file path")
	}
	migrationDir := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "db", "migrations"))
	up, err := os.ReadFile(filepath.Join(migrationDir, "000039_ugc_moderation_and_legal_acceptance.up.sql"))
	if err != nil {
		t.Fatalf("read UGC migration: %v", err)
	}
	content := string(up)
	for _, required := range []string{
		"CREATE TABLE moderation_reports",
		"moderation_reports_active_duplicate_uidx",
		"target_snapshot jsonb",
		"CREATE TABLE user_blocks",
		"CHECK (blocker_user_id <> blocked_user_id)",
		"CREATE TABLE user_legal_acceptances",
		"'moderation.manage'",
	} {
		if !strings.Contains(content, required) {
			t.Fatalf("UGC migration is missing %q", required)
		}
	}

	down, err := os.ReadFile(filepath.Join(migrationDir, "000039_ugc_moderation_and_legal_acceptance.down.sql"))
	if err != nil {
		t.Fatalf("read UGC rollback migration: %v", err)
	}
	for _, table := range []string{"user_legal_acceptances", "user_blocks", "moderation_reports"} {
		if !strings.Contains(string(down), "DROP TABLE IF EXISTS "+table) {
			t.Fatalf("UGC rollback is missing table %s", table)
		}
	}
}
