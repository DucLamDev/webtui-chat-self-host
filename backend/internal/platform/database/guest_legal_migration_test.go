package database

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGuestLegalAcceptanceMigrationRevokesLegacyGrantsWithoutBackfill(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve current test file")
	}
	migrationDir := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", "..", "..", "db", "migrations"))
	up, err := os.ReadFile(filepath.Join(migrationDir, "000040_guest_legal_acceptance_evidence.up.sql"))
	if err != nil {
		t.Fatalf("read guest legal migration: %v", err)
	}
	source := string(up)
	for _, required := range []string{
		"terms_version text",
		"privacy_policy_version text",
		"legal_accepted_at timestamptz",
		"legal_ip_address inet",
		"legal_user_agent text",
		"SET status = 'expired'",
		"legal_accepted_at IS NULL",
	} {
		if !strings.Contains(source, required) {
			t.Fatalf("guest legal migration is missing %q", required)
		}
	}
	if strings.Contains(source, "SET terms_version") || strings.Contains(source, "SET privacy_policy_version") {
		t.Fatal("migration must revoke legacy guest grants, not fabricate acceptance versions")
	}
}
