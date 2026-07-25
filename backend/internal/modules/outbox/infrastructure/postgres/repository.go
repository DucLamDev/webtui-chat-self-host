package postgres

import (
	"context"
	"time"

	outboxdomain "github.com/duclamdev/application-chat/backend/internal/modules/outbox/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Claim(ctx context.Context, limit int) ([]outboxdomain.Event, error) {
	rows, err := r.pool.Query(ctx, `
WITH picked AS (
    SELECT id
    FROM outbox_events
    WHERE status = 'pending'
       OR (status = 'failed' AND (next_attempt_at IS NULL OR next_attempt_at <= now()))
       OR (status = 'processing' AND updated_at < now() - interval '5 minutes')
    ORDER BY created_at ASC
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE outbox_events oe
SET status = 'processing'
FROM picked
WHERE oe.id = picked.id
RETURNING oe.id::text, oe.aggregate_type, oe.aggregate_id::text, oe.event_type,
          oe.event_version, oe.payload, oe.status, oe.retry_count, oe.created_at
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]outboxdomain.Event, 0)
	for rows.Next() {
		var event outboxdomain.Event
		if err := rows.Scan(
			&event.ID,
			&event.AggregateType,
			&event.AggregateID,
			&event.EventType,
			&event.EventVersion,
			&event.Payload,
			&event.Status,
			&event.RetryCount,
			&event.CreatedAt,
		); err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *Repository) MarkPublished(ctx context.Context, eventID string) error {
	_, err := r.pool.Exec(ctx, `
UPDATE outbox_events
SET status = 'published',
    published_at = now(),
    next_attempt_at = NULL
WHERE id = $1::uuid
`, eventID)
	return err
}

func (r *Repository) MarkFailed(ctx context.Context, eventID string, retryAfter time.Duration, maxRetries int, reason string) error {
	_, err := r.pool.Exec(ctx, `
UPDATE outbox_events
SET retry_count = retry_count + 1,
    status = CASE WHEN retry_count + 1 >= $3 THEN 'dead' ELSE 'failed' END,
    next_attempt_at = CASE WHEN retry_count + 1 >= $3 THEN NULL ELSE now() + make_interval(secs => $2::int) END
WHERE id = $1::uuid
`, eventID, int(retryAfter.Seconds()), maxRetries)
	return err
}
