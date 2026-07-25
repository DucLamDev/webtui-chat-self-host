package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	backupsapp "github.com/duclamdev/application-chat/backend/internal/modules/backups/application"
	backupsdomain "github.com/duclamdev/application-chat/backend/internal/modules/backups/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListJobs(ctx context.Context, workspaceID string, limit int) ([]backupsdomain.Job, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, workspace_id::text, name, target, backup_type, schedule, status, config::text,
       last_run_at, next_run_at, locked_at, locked_by, created_at, updated_at
FROM backup_jobs
WHERE workspace_id = $1::uuid OR workspace_id IS NULL
ORDER BY created_at DESC
LIMIT $2
`, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]backupsdomain.Job, 0)
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (r *Repository) GetJob(ctx context.Context, jobID string) (backupsdomain.Job, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id::text, workspace_id::text, name, target, backup_type, schedule, status, config::text,
       last_run_at, next_run_at, locked_at, locked_by, created_at, updated_at
FROM backup_jobs
WHERE id = $1::uuid
`, jobID)
	return scanJob(row)
}

func (r *Repository) CreateJob(ctx context.Context, params backupsapp.SaveJobParams) (backupsdomain.Job, error) {
	row := r.pool.QueryRow(ctx, `
INSERT INTO backup_jobs (workspace_id, name, target, backup_type, schedule, status, config, next_run_at)
VALUES ($1::uuid, $2, $3, $4, NULLIF($5, ''), $6, $7::jsonb, $8)
RETURNING id::text, workspace_id::text, name, target, backup_type, schedule, status, config::text,
          last_run_at, next_run_at, locked_at, locked_by, created_at, updated_at
`, params.WorkspaceID, params.Name, params.Target, params.BackupType, params.Schedule, params.Status, string(params.Config), params.NextRunAt)
	return scanJob(row)
}

func (r *Repository) ListRuns(ctx context.Context, jobID string, limit int) ([]backupsdomain.Run, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, backup_job_id::text, status, backup_type, object_key, byte_size, checksum_sha256,
       started_at, finished_at, error
FROM backup_runs
WHERE backup_job_id = $1::uuid
ORDER BY started_at DESC
LIMIT $2
`, jobID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := make([]backupsdomain.Run, 0)
	for rows.Next() {
		run, err := scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (r *Repository) ClaimDue(ctx context.Context, limit int, nodeID string, lockTTL time.Duration) ([]backupsdomain.Job, error) {
	rows, err := r.pool.Query(ctx, `
WITH picked AS (
    SELECT id
    FROM backup_jobs
    WHERE status = 'active'
      AND next_run_at IS NOT NULL
      AND next_run_at <= now()
      AND (locked_at IS NULL OR locked_at < now() - make_interval(secs => $3::int))
    ORDER BY next_run_at ASC
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE backup_jobs bj
SET locked_at = now(),
    locked_by = $2
FROM picked
WHERE bj.id = picked.id
RETURNING bj.id::text, bj.workspace_id::text, bj.name, bj.target, bj.backup_type, bj.schedule, bj.status, bj.config::text,
          bj.last_run_at, bj.next_run_at, bj.locked_at, bj.locked_by, bj.created_at, bj.updated_at
`, limit, nodeID, int(lockTTL.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]backupsdomain.Job, 0)
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (r *Repository) StartRun(ctx context.Context, jobID string, backupType string) (backupsdomain.Run, error) {
	row := r.pool.QueryRow(ctx, `
INSERT INTO backup_runs (backup_job_id, status, backup_type)
VALUES ($1::uuid, 'running', $2)
RETURNING id::text, backup_job_id::text, status, backup_type, object_key, byte_size, checksum_sha256,
          started_at, finished_at, error
`, jobID, backupType)
	return scanRun(row)
}

func (r *Repository) FinishRun(ctx context.Context, params backupsapp.FinishRunParams) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `
UPDATE backup_runs
SET status = $2,
    object_key = NULLIF($3, ''),
    byte_size = $4,
    checksum_sha256 = NULLIF($5, ''),
    finished_at = now(),
    error = NULLIF($6, '')
WHERE id = $1::uuid
`, params.RunID, params.Status, params.ObjectKey, params.ByteSize, params.Checksum, params.Error); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
UPDATE backup_jobs
SET last_run_at = now(),
    next_run_at = $2,
    locked_at = NULL,
    locked_by = NULL
WHERE id = $1::uuid
`, params.JobID, params.NextRunAt); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanJob(row rowScanner) (backupsdomain.Job, error) {
	var job backupsdomain.Job
	var workspaceID sql.NullString
	var schedule sql.NullString
	var config string
	var lastRunAt sql.NullTime
	var nextRunAt sql.NullTime
	var lockedAt sql.NullTime
	var lockedBy sql.NullString
	if err := row.Scan(
		&job.ID,
		&workspaceID,
		&job.Name,
		&job.Target,
		&job.BackupType,
		&schedule,
		&job.Status,
		&config,
		&lastRunAt,
		&nextRunAt,
		&lockedAt,
		&lockedBy,
		&job.CreatedAt,
		&job.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return backupsdomain.Job{}, backupsdomain.ErrBackupJobNotFound
		}
		return backupsdomain.Job{}, err
	}
	job.WorkspaceID = nullStringPtr(workspaceID)
	job.Schedule = nullStringPtr(schedule)
	job.Config = []byte(config)
	job.LastRunAt = nullTimePtr(lastRunAt)
	job.NextRunAt = nullTimePtr(nextRunAt)
	job.LockedAt = nullTimePtr(lockedAt)
	job.LockedBy = nullStringPtr(lockedBy)
	return job, nil
}

func scanRun(row rowScanner) (backupsdomain.Run, error) {
	var run backupsdomain.Run
	var backupJobID sql.NullString
	var objectKey sql.NullString
	var byteSize sql.NullInt64
	var checksum sql.NullString
	var finishedAt sql.NullTime
	var errorText sql.NullString
	if err := row.Scan(
		&run.ID,
		&backupJobID,
		&run.Status,
		&run.BackupType,
		&objectKey,
		&byteSize,
		&checksum,
		&run.StartedAt,
		&finishedAt,
		&errorText,
	); err != nil {
		return backupsdomain.Run{}, err
	}
	run.BackupJobID = nullStringPtr(backupJobID)
	run.ObjectKey = nullStringPtr(objectKey)
	run.ByteSize = nullIntPtr(byteSize)
	run.Checksum = nullStringPtr(checksum)
	run.FinishedAt = nullTimePtr(finishedAt)
	run.Error = nullStringPtr(errorText)
	return run, nil
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func nullIntPtr(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	return &value.Int64
}
