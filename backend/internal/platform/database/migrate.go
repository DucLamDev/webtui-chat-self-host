package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
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
