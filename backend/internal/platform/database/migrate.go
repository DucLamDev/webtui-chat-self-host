package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	migrationNoTransactionDirective       = "-- migrate:no-transaction"
	migrationStatementBreakpoint          = "-- migrate:statement-breakpoint"
	migrationAdvisoryLockKey        int64 = 0x5754425455494d47
	migrationLockWaitTimeout              = 30 * time.Second
)

type MigrationRunner struct {
	db   *Postgres
	path string
}

func NewMigrationRunner(db *Postgres, path string) *MigrationRunner {
	return &MigrationRunner{
		db:   db,
		path: path,
	}
}

func (r *MigrationRunner) Up(ctx context.Context) error {
	if r.db == nil || r.db.pool == nil {
		return ErrDisabled
	}

	if err := r.ensureTable(ctx); err != nil {
		return err
	}
	releaseMigrationLock, err := r.acquireMigrationLock(ctx)
	if err != nil {
		return err
	}
	defer releaseMigrationLock()

	files, err := r.upFiles()
	if err != nil {
		return err
	}

	for _, file := range files {
		version := migrationVersion(file)
		applied, err := r.applied(ctx, version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("đọc migration %s: %w", file, err)
		}

		statements, noTransaction, err := nonTransactionalMigrationStatements(string(sqlBytes))
		if err != nil {
			return fmt.Errorf("parse migration %s: %w", file, err)
		}
		if noTransaction {
			if err := r.runNonTransactionalUp(ctx, file, version, statements); err != nil {
				return err
			}
			continue
		}

		if err := r.db.Tx(ctx, func(txCtx context.Context, tx pgx.Tx) error {
			if _, err := tx.Exec(txCtx, string(sqlBytes)); err != nil {
				return fmt.Errorf("chạy migration %s: %w", file, err)
			}
			if _, err := tx.Exec(txCtx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
				return fmt.Errorf("ghi nhận migration %s: %w", file, err)
			}
			return nil
		}); err != nil {
			return err
		}
	}

	return nil
}

// Down rolls back the most recently applied migrations one at a time.
func (r *MigrationRunner) Down(ctx context.Context, steps int) error {
	if r.db == nil || r.db.pool == nil {
		return ErrDisabled
	}
	if steps <= 0 {
		return errors.New("migration rollback steps must be greater than zero")
	}
	if err := r.ensureTable(ctx); err != nil {
		return err
	}
	releaseMigrationLock, err := r.acquireMigrationLock(ctx)
	if err != nil {
		return err
	}
	defer releaseMigrationLock()

	rows, err := r.db.pool.Query(ctx, `
SELECT version
FROM schema_migrations
ORDER BY version DESC
LIMIT $1
`, steps)
	if err != nil {
		return fmt.Errorf("list applied migrations for rollback: %w", err)
	}
	versions := make([]string, 0, steps)
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			rows.Close()
			return fmt.Errorf("scan applied migration for rollback: %w", err)
		}
		versions = append(versions, version)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("list applied migrations for rollback: %w", err)
	}
	rows.Close()

	for _, version := range versions {
		file := filepath.Join(r.path, version+".down.sql")
		sqlBytes, err := os.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read rollback migration %s: %w", file, err)
		}
		statements, noTransaction, err := nonTransactionalMigrationStatements(string(sqlBytes))
		if err != nil {
			return fmt.Errorf("parse rollback migration %s: %w", file, err)
		}
		if noTransaction {
			if err := r.runNonTransactionalDown(ctx, file, version, statements); err != nil {
				return err
			}
			continue
		}

		if err := r.db.Tx(ctx, func(txCtx context.Context, tx pgx.Tx) error {
			if _, err := tx.Exec(txCtx, string(sqlBytes)); err != nil {
				return fmt.Errorf("run rollback migration %s: %w", file, err)
			}
			if _, err := tx.Exec(txCtx, "DELETE FROM schema_migrations WHERE version = $1", version); err != nil {
				return fmt.Errorf("remove migration record %s: %w", version, err)
			}
			return nil
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *MigrationRunner) ensureTable(ctx context.Context) error {
	_, err := r.db.pool.Exec(ctx, `
CREATE TABLE IF NOT EXISTS schema_migrations (
    version text PRIMARY KEY,
    applied_at timestamptz NOT NULL DEFAULT now()
)`)
	if err != nil {
		return fmt.Errorf("đảm bảo bảng schema_migrations tồn tại: %w", err)
	}
	return nil
}

func (r *MigrationRunner) applied(ctx context.Context, version string) (bool, error) {
	var exists bool
	err := r.db.pool.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("kiểm tra migration %s: %w", version, err)
	}
	return exists, nil
}

func (r *MigrationRunner) upFiles() ([]string, error) {
	pattern := filepath.Join(r.path, "*.up.sql")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("tìm danh sách migration: %w", err)
	}
	sort.Strings(files)
	return files, nil
}

func migrationVersion(path string) string {
	name := filepath.Base(path)
	return strings.TrimSuffix(name, ".up.sql")
}

func (r *MigrationRunner) acquireMigrationLock(ctx context.Context) (func(), error) {
	conn, err := r.db.pool.Acquire(ctx)
	if err != nil {
		return nil, fmt.Errorf("acquire schema migration lock connection: %w", err)
	}
	destroyConnection := func() {
		destroyPooledConnection(conn)
	}
	lockCtx, cancelLock := context.WithTimeout(ctx, migrationLockWaitTimeout)
	defer cancelLock()
	if _, err := conn.Exec(lockCtx, "SELECT pg_advisory_lock($1)", migrationAdvisoryLockKey); err != nil {
		// Closing the session is the only safe cleanup if cancellation races with
		// lock acquisition; PostgreSQL releases every session advisory lock.
		destroyConnection()
		return nil, fmt.Errorf("acquire schema migration advisory lock: %w", err)
	}

	return func() {
		unlockCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var unlocked bool
		if err := conn.QueryRow(unlockCtx, "SELECT pg_advisory_unlock($1)", migrationAdvisoryLockKey).Scan(&unlocked); err == nil && unlocked {
			conn.Release()
			return
		}
		// Never return a connection that might still hold the global lock to the
		// pool. Destroying its PostgreSQL session guarantees lock release.
		destroyConnection()
	}, nil
}

func (r *MigrationRunner) runNonTransactionalUp(ctx context.Context, file string, version string, statements []string) error {
	conn, err := r.db.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection for non-transactional migration %s: %w", file, err)
	}
	// Destroy this session after the migration so SET timeouts and any unknown
	// session state can never leak to another pool borrower.
	defer destroyPooledConnection(conn)

	for index, statement := range statements {
		if _, err := conn.Exec(ctx, statement); err != nil {
			return fmt.Errorf("run non-transactional migration %s statement %d: %w", file, index+1, err)
		}
	}
	if _, err := conn.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
		return fmt.Errorf("record non-transactional migration %s: %w", file, err)
	}
	return nil
}

func (r *MigrationRunner) runNonTransactionalDown(ctx context.Context, file string, version string, statements []string) error {
	conn, err := r.db.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection for non-transactional rollback %s: %w", file, err)
	}
	// Destroy this session after the rollback so SET timeouts and any unknown
	// session state can never leak to another pool borrower.
	defer destroyPooledConnection(conn)

	for index, statement := range statements {
		if _, err := conn.Exec(ctx, statement); err != nil {
			return fmt.Errorf("run non-transactional rollback %s statement %d: %w", file, index+1, err)
		}
	}
	if _, err := conn.Exec(ctx, "DELETE FROM schema_migrations WHERE version = $1", version); err != nil {
		return fmt.Errorf("remove non-transactional migration record %s: %w", file, err)
	}
	return nil
}

func nonTransactionalMigrationStatements(sqlText string) ([]string, bool, error) {
	normalized := strings.ReplaceAll(sqlText, "\r\n", "\n")
	firstMeaningfulLine := ""
	for _, line := range strings.Split(normalized, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			firstMeaningfulLine = trimmed
			break
		}
	}
	if firstMeaningfulLine != migrationNoTransactionDirective {
		return nil, false, nil
	}

	normalized = strings.Replace(normalized, migrationNoTransactionDirective, "", 1)
	parts := strings.Split(normalized, migrationStatementBreakpoint)
	statements := make([]string, 0, len(parts))
	for _, part := range parts {
		statement := strings.TrimSpace(part)
		if statement == "" {
			return nil, true, errors.New("non-transactional migration contains an empty statement")
		}
		// Sending multiple statements in one PostgreSQL query creates an implicit
		// transaction, which would make CONCURRENTLY fail. The explicit marker is
		// therefore mandatory between every statement in this migration mode.
		if !strings.HasSuffix(statement, ";") || strings.Count(statement, ";") != 1 {
			return nil, true, errors.New("each non-transactional migration block must contain exactly one semicolon-terminated statement")
		}
		statements = append(statements, statement)
	}
	if len(statements) == 0 {
		return nil, true, errors.New("non-transactional migration must contain at least one statement")
	}
	return statements, true, nil
}

func destroyPooledConnection(conn *pgxpool.Conn) {
	rawConnection := conn.Hijack()
	closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = rawConnection.Close(closeCtx)
}
