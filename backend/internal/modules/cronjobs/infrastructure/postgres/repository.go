package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	cronjobsapp "github.com/duclamdev/application-chat/backend/internal/modules/cronjobs/application"
	cronjobsdomain "github.com/duclamdev/application-chat/backend/internal/modules/cronjobs/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) List(ctx context.Context, status string, limit int) ([]cronjobsdomain.CronJob, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, name, description, schedule, runner, status, payload::text,
       last_run_at, next_run_at, locked_at, locked_by, created_at, updated_at
FROM cron_jobs
WHERE ($1 = '' OR status = $1)
ORDER BY next_run_at NULLS LAST, name
LIMIT $2
`, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]cronjobsdomain.CronJob, 0)
	for rows.Next() {
		job, err := scanCronJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (r *Repository) Get(ctx context.Context, jobID string) (cronjobsdomain.CronJob, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id::text, name, description, schedule, runner, status, payload::text,
       last_run_at, next_run_at, locked_at, locked_by, created_at, updated_at
FROM cron_jobs
WHERE id = $1::uuid
`, jobID)
	return scanCronJob(row)
}

func (r *Repository) Create(ctx context.Context, params cronjobsapp.SaveCronJobParams) (cronjobsdomain.CronJob, error) {
	row := r.pool.QueryRow(ctx, `
INSERT INTO cron_jobs (name, description, schedule, runner, status, payload, next_run_at)
VALUES ($1, NULLIF($2, ''), $3, $4, $5, $6::jsonb, $7)
RETURNING id::text, name, description, schedule, runner, status, payload::text,
          last_run_at, next_run_at, locked_at, locked_by, created_at, updated_at
`, params.Name, params.Description, params.Schedule, params.Runner, params.Status, string(params.Payload), params.NextRunAt)
	job, err := scanCronJob(row)
	if isUniqueViolation(err) {
		return cronjobsdomain.CronJob{}, cronjobsdomain.ErrCronJobAlreadyExists
	}
	return job, err
}

func (r *Repository) Update(ctx context.Context, params cronjobsapp.SaveCronJobParams) (cronjobsdomain.CronJob, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE cron_jobs
SET name = $2,
    description = NULLIF($3, ''),
    schedule = $4,
    runner = $5,
    status = $6,
    payload = $7::jsonb,
    next_run_at = $8,
    locked_at = NULL,
    locked_by = NULL
WHERE id = $1::uuid
RETURNING id::text, name, description, schedule, runner, status, payload::text,
          last_run_at, next_run_at, locked_at, locked_by, created_at, updated_at
`, params.ID, params.Name, params.Description, params.Schedule, params.Runner, params.Status, string(params.Payload), params.NextRunAt)
	job, err := scanCronJob(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return cronjobsdomain.CronJob{}, cronjobsdomain.ErrCronJobNotFound
	}
	if isUniqueViolation(err) {
		return cronjobsdomain.CronJob{}, cronjobsdomain.ErrCronJobAlreadyExists
	}
	return job, err
}

func (r *Repository) Delete(ctx context.Context, jobID string) error {
	command, err := r.pool.Exec(ctx, `
DELETE FROM cron_jobs
WHERE id = $1::uuid
`, jobID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return cronjobsdomain.ErrCronJobNotFound
	}
	return nil
}

func (r *Repository) ListRuns(ctx context.Context, jobID string, limit int) ([]cronjobsdomain.CronJobRun, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, cron_job_id::text, status, started_at, finished_at, log, error
FROM cron_job_runs
WHERE cron_job_id = $1::uuid
ORDER BY started_at DESC
LIMIT $2
`, jobID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := make([]cronjobsdomain.CronJobRun, 0)
	for rows.Next() {
		run, err := scanCronJobRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (r *Repository) ClaimDue(ctx context.Context, limit int, nodeID string, lockTTL time.Duration) ([]cronjobsdomain.CronJob, error) {
	rows, err := r.pool.Query(ctx, `
WITH picked AS (
    SELECT id
    FROM cron_jobs
    WHERE status = 'active'
      AND next_run_at IS NOT NULL
      AND next_run_at <= now()
      AND (locked_at IS NULL OR locked_at < now() - make_interval(secs => $3::int))
    ORDER BY next_run_at ASC
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE cron_jobs cj
SET locked_at = now(),
    locked_by = $2
FROM picked
WHERE cj.id = picked.id
RETURNING cj.id::text, cj.name, cj.description, cj.schedule, cj.runner, cj.status, cj.payload::text,
          cj.last_run_at, cj.next_run_at, cj.locked_at, cj.locked_by, cj.created_at, cj.updated_at
`, limit, nodeID, int(lockTTL.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	jobs := make([]cronjobsdomain.CronJob, 0)
	for rows.Next() {
		job, err := scanCronJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job)
	}
	return jobs, rows.Err()
}

func (r *Repository) StartRun(ctx context.Context, jobID string) (cronjobsdomain.CronJobRun, error) {
	row := r.pool.QueryRow(ctx, `
INSERT INTO cron_job_runs (cron_job_id, status)
VALUES ($1::uuid, 'running')
RETURNING id::text, cron_job_id::text, status, started_at, finished_at, log, error
`, jobID)
	run, err := scanCronJobRun(row)
	if err != nil {
		return cronjobsdomain.CronJobRun{}, err
	}
	return run, nil
}

func (r *Repository) FinishRun(ctx context.Context, params cronjobsapp.FinishRunParams) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `
UPDATE cron_job_runs
SET status = $2,
    finished_at = now(),
    log = NULLIF($3, ''),
    error = NULLIF($4, '')
WHERE id = $1::uuid
`, params.RunID, params.Status, params.Log, params.Error); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
UPDATE cron_jobs
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

func (r *Repository) CleanupExpiredSessions(ctx context.Context, olderThan time.Duration) (int, error) {
	command, err := r.pool.Exec(ctx, `
DELETE FROM user_sessions
WHERE expires_at < now() - make_interval(secs => $1::int)
   OR (revoked_at IS NOT NULL AND revoked_at < now() - make_interval(secs => $1::int))
`, int(olderThan.Seconds()))
	if err != nil {
		return 0, err
	}
	return int(command.RowsAffected()), nil
}

func (r *Repository) CleanupDeadOutbox(ctx context.Context, olderThan time.Duration) (int, error) {
	command, err := r.pool.Exec(ctx, `
DELETE FROM outbox_events
WHERE status IN ('published', 'dead')
  AND updated_at < now() - make_interval(secs => $1::int)
`, int(olderThan.Seconds()))
	if err != nil {
		return 0, err
	}
	return int(command.RowsAffected()), nil
}

func (r *Repository) CleanupFailedUploads(ctx context.Context, olderThan time.Duration) (int, error) {
	command, err := r.pool.Exec(ctx, `
UPDATE files
SET status = 'deleted',
    deleted_at = COALESCE(deleted_at, now())
WHERE status IN ('uploading', 'failed')
  AND deleted_at IS NULL
  AND created_at < now() - make_interval(secs => $1::int)
`, int(olderThan.Seconds()))
	if err != nil {
		return 0, err
	}
	return int(command.RowsAffected()), nil
}

func (r *Repository) CleanupOldPresence(ctx context.Context, olderThan time.Duration) (int, error) {
	command, err := r.pool.Exec(ctx, `
DELETE FROM user_presence
WHERE status = 'offline'
  AND last_heartbeat_at < now() - make_interval(secs => $1::int)
`, int(olderThan.Seconds()))
	if err != nil {
		return 0, err
	}
	return int(command.RowsAffected()), nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCronJob(row rowScanner) (cronjobsdomain.CronJob, error) {
	var job cronjobsdomain.CronJob
	var description sql.NullString
	var payload string
	var lastRunAt sql.NullTime
	var nextRunAt sql.NullTime
	var lockedAt sql.NullTime
	var lockedBy sql.NullString
	if err := row.Scan(
		&job.ID,
		&job.Name,
		&description,
		&job.Schedule,
		&job.Runner,
		&job.Status,
		&payload,
		&lastRunAt,
		&nextRunAt,
		&lockedAt,
		&lockedBy,
		&job.CreatedAt,
		&job.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return cronjobsdomain.CronJob{}, cronjobsdomain.ErrCronJobNotFound
		}
		return cronjobsdomain.CronJob{}, err
	}
	job.Description = nullStringPtr(description)
	job.Payload = []byte(payload)
	job.LastRunAt = nullTimePtr(lastRunAt)
	job.NextRunAt = nullTimePtr(nextRunAt)
	job.LockedAt = nullTimePtr(lockedAt)
	job.LockedBy = nullStringPtr(lockedBy)
	return job, nil
}

func scanCronJobRun(row rowScanner) (cronjobsdomain.CronJobRun, error) {
	var run cronjobsdomain.CronJobRun
	var finishedAt sql.NullTime
	var logText sql.NullString
	var errorText sql.NullString
	if err := row.Scan(
		&run.ID,
		&run.CronJobID,
		&run.Status,
		&run.StartedAt,
		&finishedAt,
		&logText,
		&errorText,
	); err != nil {
		return cronjobsdomain.CronJobRun{}, err
	}
	run.FinishedAt = nullTimePtr(finishedAt)
	run.Log = nullStringPtr(logText)
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

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
