package relayserver

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNoJobs              = errors.New("no relay jobs are due")
	ErrJobNotFound         = errors.New("relay job not found")
	ErrIdempotencyConflict = errors.New("idempotency key was reused with a different request")
)

type EnqueueInput struct {
	PublisherID    string
	IdempotencyKey string
	RequestHash    []byte
	Provider       string
	DeviceToken    string
	Payload        []byte
	MaxAttempts    int
}

type Job struct {
	ID           string     `json:"id"`
	PublisherID  string     `json:"-"`
	Provider     string     `json:"provider"`
	DeviceToken  string     `json:"-"`
	Payload      []byte     `json:"-"`
	Status       string     `json:"status"`
	AttemptCount int        `json:"attempt_count"`
	MaxAttempts  int        `json:"-"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	DeliveredAt  *time.Time `json:"delivered_at,omitempty"`
}

// QueueStats is an aggregate-only operational snapshot. It deliberately does
// not contain publisher IDs, destination tokens, payloads, or provider errors
// so it is safe to expose on the relay's internal Prometheus endpoint.
type QueueStats struct {
	Pending                int64
	Processing             int64
	Retry                  int64
	Sent                   int64
	Dead                   int64
	Sent24Hours            int64
	Dead24Hours            int64
	OldestQueuedAgeSeconds float64
}

type Store interface {
	Enqueue(context.Context, EnqueueInput) (Job, bool, error)
	Get(context.Context, string, string) (Job, error)
	Claim(context.Context) (Job, error)
	MarkSent(context.Context, string) error
	MarkFailed(context.Context, Job, bool, string, time.Time) error
	ReapExpiredProcessing(context.Context, time.Time) (int64, error)
	Stats(context.Context, time.Time) (QueueStats, error)
	Ping(context.Context) error
	Purge(context.Context, time.Time) (int64, error)
}

type PostgresStore struct {
	pool *pgxpool.Pool
}

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore {
	return &PostgresStore{pool: pool}
}

func (s *PostgresStore) Enqueue(ctx context.Context, input EnqueueInput) (Job, bool, error) {
	var job Job
	err := s.pool.QueryRow(ctx, `
INSERT INTO push_relay_jobs (
    publisher_id, idempotency_key, request_hash, provider, device_token,
    payload, max_attempts
)
VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)
ON CONFLICT (publisher_id, idempotency_key) DO NOTHING
RETURNING id::text, publisher_id, provider, status, attempt_count,
          max_attempts, created_at, updated_at, delivered_at
`, input.PublisherID, input.IdempotencyKey, input.RequestHash, input.Provider,
		input.DeviceToken, string(input.Payload), input.MaxAttempts).Scan(
		&job.ID, &job.PublisherID, &job.Provider, &job.Status, &job.AttemptCount,
		&job.MaxAttempts, &job.CreatedAt, &job.UpdatedAt, &job.DeliveredAt,
	)
	if err == nil {
		return job, false, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Job{}, false, err
	}
	var existingHash []byte
	err = s.pool.QueryRow(ctx, `
SELECT id::text, publisher_id, provider, status, attempt_count,
       max_attempts, created_at, updated_at, delivered_at, request_hash
FROM push_relay_jobs
WHERE publisher_id = $1 AND idempotency_key = $2
`, input.PublisherID, input.IdempotencyKey).Scan(
		&job.ID, &job.PublisherID, &job.Provider, &job.Status, &job.AttemptCount,
		&job.MaxAttempts, &job.CreatedAt, &job.UpdatedAt, &job.DeliveredAt, &existingHash,
	)
	if err != nil {
		return Job{}, false, err
	}
	if len(existingHash) != len(input.RequestHash) || subtle.ConstantTimeCompare(existingHash, input.RequestHash) != 1 {
		return Job{}, false, ErrIdempotencyConflict
	}
	return job, true, nil
}

func (s *PostgresStore) Get(ctx context.Context, publisherID string, jobID string) (Job, error) {
	var job Job
	err := s.pool.QueryRow(ctx, `
SELECT id::text, publisher_id, provider, status, attempt_count,
       max_attempts, created_at, updated_at, delivered_at
FROM push_relay_jobs
WHERE id = $2::uuid AND publisher_id = $1
`, publisherID, jobID).Scan(
		&job.ID, &job.PublisherID, &job.Provider, &job.Status, &job.AttemptCount,
		&job.MaxAttempts, &job.CreatedAt, &job.UpdatedAt, &job.DeliveredAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrJobNotFound
	}
	return job, err
}

func (s *PostgresStore) Claim(ctx context.Context) (Job, error) {
	var job Job
	var payload string
	err := s.pool.QueryRow(ctx, `
WITH candidate AS (
    SELECT id
    FROM push_relay_jobs
    WHERE (
        (status IN ('pending', 'retry') AND next_attempt_at <= now())
        OR (status = 'processing' AND leased_at <= now() - interval '2 minutes')
    )
      AND attempt_count < max_attempts
    ORDER BY next_attempt_at, created_at
    LIMIT 1
    FOR UPDATE SKIP LOCKED
), claimed AS (
    UPDATE push_relay_jobs job
    SET status = 'processing',
        attempt_count = attempt_count + 1,
        leased_at = now(),
        last_error = NULL
    FROM candidate
    WHERE job.id = candidate.id
    RETURNING job.id::text, job.publisher_id, job.provider, job.device_token,
              job.payload::text, job.status, job.attempt_count,
              job.max_attempts, job.created_at, job.updated_at, job.delivered_at
)
SELECT id, publisher_id, provider, device_token, payload, status,
       attempt_count, max_attempts, created_at, updated_at, delivered_at
FROM claimed
`).Scan(
		&job.ID, &job.PublisherID, &job.Provider, &job.DeviceToken, &payload,
		&job.Status, &job.AttemptCount, &job.MaxAttempts,
		&job.CreatedAt, &job.UpdatedAt, &job.DeliveredAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Job{}, ErrNoJobs
	}
	job.Payload = []byte(payload)
	return job, err
}

func (s *PostgresStore) MarkSent(ctx context.Context, jobID string) error {
	_, err := s.pool.Exec(ctx, `
UPDATE push_relay_jobs
SET status = 'sent', delivered_at = now(), leased_at = NULL,
    last_error = NULL, device_token = '[delivered]', payload = '{}'::jsonb
WHERE id = $1::uuid AND status = 'processing'
`, jobID)
	return err
}

func (s *PostgresStore) MarkFailed(ctx context.Context, job Job, permanent bool, reason string, retryAt time.Time) error {
	dead := permanent || job.AttemptCount >= job.MaxAttempts
	_, err := s.pool.Exec(ctx, `
UPDATE push_relay_jobs
SET status = CASE WHEN $2 THEN 'dead' ELSE 'retry' END,
    next_attempt_at = CASE WHEN $2 THEN next_attempt_at ELSE $4 END,
    leased_at = NULL,
    last_error = left($3, 500),
    device_token = CASE WHEN $2 THEN '[discarded]' ELSE device_token END,
    payload = CASE WHEN $2 THEN '{}'::jsonb ELSE payload END
WHERE id = $1::uuid AND status = 'processing'
`, job.ID, dead, reason, retryAt)
	return err
}

// ReapExpiredProcessing closes the crash window on the final provider
// attempt. A job claimed for its last attempt cannot be reclaimed by Claim
// because its attempt budget is exhausted. If the process exits before
// MarkSent/MarkFailed commits, move that expired lease to dead-letter and
// discard all delivery credentials instead of leaving it in processing
// forever.
func (s *PostgresStore) ReapExpiredProcessing(ctx context.Context, leasedBefore time.Time) (int64, error) {
	command, err := s.pool.Exec(ctx, `
UPDATE push_relay_jobs
SET status = 'dead',
    next_attempt_at = NULL,
    leased_at = NULL,
    delivered_at = NULL,
    last_error = 'delivery lease expired after final attempt',
    device_token = '[discarded]',
    payload = '{}'::jsonb
WHERE status = 'processing'
  AND leased_at IS NOT NULL
  AND leased_at <= $1
  AND attempt_count >= max_attempts
`, leasedBefore)
	if err != nil {
		return 0, err
	}
	return command.RowsAffected(), nil
}

func (s *PostgresStore) Stats(ctx context.Context, at time.Time) (QueueStats, error) {
	var stats QueueStats
	err := s.pool.QueryRow(ctx, `
SELECT
    count(*) FILTER (WHERE status = 'pending'),
    count(*) FILTER (WHERE status = 'processing'),
    count(*) FILTER (WHERE status = 'retry'),
    count(*) FILTER (WHERE status = 'sent'),
    count(*) FILTER (WHERE status = 'dead'),
    count(*) FILTER (
        WHERE status = 'sent'
          AND delivered_at >= $1::timestamptz - interval '24 hours'
    ),
    count(*) FILTER (
        WHERE status = 'dead'
          AND updated_at >= $1::timestamptz - interval '24 hours'
    ),
    GREATEST(0, COALESCE(extract(epoch FROM (
        $1::timestamptz - min(created_at) FILTER (
            WHERE status IN ('pending', 'processing', 'retry')
        )
    )), 0))
FROM push_relay_jobs
`, at).Scan(
		&stats.Pending,
		&stats.Processing,
		&stats.Retry,
		&stats.Sent,
		&stats.Dead,
		&stats.Sent24Hours,
		&stats.Dead24Hours,
		&stats.OldestQueuedAgeSeconds,
	)
	return stats, err
}

func (s *PostgresStore) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *PostgresStore) Purge(ctx context.Context, before time.Time) (int64, error) {
	command, err := s.pool.Exec(ctx, `
DELETE FROM push_relay_jobs
WHERE status IN ('sent', 'dead') AND updated_at < $1
`, before)
	if err != nil {
		return 0, err
	}
	return command.RowsAffected(), nil
}
