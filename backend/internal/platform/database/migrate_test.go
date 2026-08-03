package database

import (
	"strings"
	"testing"
)

func TestNonTransactionalMigrationStatements(t *testing.T) {
	sqlText := `-- migrate:no-transaction
DROP INDEX CONCURRENTLY IF EXISTS example_idx;
-- migrate:statement-breakpoint
CREATE INDEX CONCURRENTLY example_idx ON example_table (created_at);`

	statements, noTransaction, err := nonTransactionalMigrationStatements(sqlText)
	if err != nil {
		t.Fatalf("nonTransactionalMigrationStatements() error = %v", err)
	}
	if !noTransaction {
		t.Fatal("nonTransactionalMigrationStatements() noTransaction = false, want true")
	}
	if len(statements) != 2 {
		t.Fatalf("len(statements) = %d, want 2", len(statements))
	}
	if !strings.HasPrefix(statements[0], "DROP INDEX CONCURRENTLY") {
		t.Fatalf("first statement = %q", statements[0])
	}
	if !strings.HasPrefix(statements[1], "CREATE INDEX CONCURRENTLY") {
		t.Fatalf("second statement = %q", statements[1])
	}
}

func TestTransactionalMigrationIsUnchanged(t *testing.T) {
	statements, noTransaction, err := nonTransactionalMigrationStatements("CREATE TABLE example (id uuid);")
	if err != nil {
		t.Fatalf("nonTransactionalMigrationStatements() error = %v", err)
	}
	if noTransaction || statements != nil {
		t.Fatalf("got statements=%v noTransaction=%v, want transactional migration", statements, noTransaction)
	}
}

func TestNonTransactionalMigrationRejectsEmptyStatement(t *testing.T) {
	_, noTransaction, err := nonTransactionalMigrationStatements(`-- migrate:no-transaction
SELECT 1;
-- migrate:statement-breakpoint`)
	if !noTransaction || err == nil {
		t.Fatalf("got noTransaction=%v error=%v, want empty statement rejection", noTransaction, err)
	}
}

func TestNonTransactionalMigrationRejectsMultipleStatementsInOneBlock(t *testing.T) {
	_, noTransaction, err := nonTransactionalMigrationStatements(`-- migrate:no-transaction
SELECT 1;
SELECT 2;`)
	if !noTransaction || err == nil {
		t.Fatalf("got noTransaction=%v error=%v, want missing breakpoint rejection", noTransaction, err)
	}
}
