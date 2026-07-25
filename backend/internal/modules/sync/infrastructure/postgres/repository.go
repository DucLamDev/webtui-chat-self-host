package postgres

import (
	"context"
	"database/sql"
	"errors"

	syncapp "github.com/duclamdev/application-chat/backend/internal/modules/sync/application"
	syncdomain "github.com/duclamdev/application-chat/backend/internal/modules/sync/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListEvents(ctx context.Context, params syncapp.ListParams) ([]syncdomain.Event, error) {
	rows, err := r.pool.Query(ctx, `
WITH cursor_event AS (
    SELECT created_at, id
    FROM outbox_events
    WHERE id = NULLIF($2, '')::uuid
)
SELECT id::text, aggregate_type, aggregate_id::text, event_type, event_version, payload::text, created_at
FROM outbox_events oe
WHERE oe.payload ->> 'workspace_id' = $1
  AND (
      $2 = ''
      OR EXISTS (
          SELECT 1
          FROM cursor_event ce
          WHERE (oe.created_at, oe.id) > (ce.created_at, ce.id)
      )
  )
ORDER BY oe.created_at ASC, oe.id ASC
LIMIT $3
`, params.WorkspaceID, params.Cursor, params.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]syncdomain.Event, 0)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (r *Repository) Ack(ctx context.Context, params syncapp.AckParams) (syncdomain.CursorAck, error) {
	row := r.pool.QueryRow(ctx, `
WITH cursor_event AS (
    SELECT id, created_at
    FROM outbox_events
    WHERE id = $4::uuid
      AND payload ->> 'workspace_id' = $2
),
upserted AS (
    INSERT INTO mobile_sync_cursors (user_id, workspace_id, device_id, cursor_event_id, cursor_created_at, acked_at)
    SELECT $1::uuid, $2::uuid, $3, id, created_at, now()
    FROM cursor_event
    ON CONFLICT (user_id, workspace_id, device_id) DO UPDATE SET
        cursor_event_id = EXCLUDED.cursor_event_id,
        cursor_created_at = EXCLUDED.cursor_created_at,
        acked_at = now()
    RETURNING user_id::text, workspace_id::text, device_id, cursor_event_id::text, cursor_created_at, acked_at
)
SELECT user_id, workspace_id, device_id, cursor_event_id, cursor_created_at, acked_at
FROM upserted
`, params.UserID, params.WorkspaceID, params.DeviceID, params.Cursor)
	return scanAck(row)
}

func (r *Repository) GetAckCursor(ctx context.Context, userID string, workspaceID string, deviceID string) (string, error) {
	var cursor sql.NullString
	err := r.pool.QueryRow(ctx, `
SELECT cursor_event_id::text
FROM mobile_sync_cursors
WHERE user_id = $1::uuid
  AND workspace_id = $2::uuid
  AND device_id = $3
`, userID, workspaceID, deviceID).Scan(&cursor)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !cursor.Valid {
		return "", nil
	}
	return cursor.String, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEvent(row rowScanner) (syncdomain.Event, error) {
	var event syncdomain.Event
	var payload string
	if err := row.Scan(
		&event.ID,
		&event.AggregateType,
		&event.AggregateID,
		&event.EventType,
		&event.EventVersion,
		&payload,
		&event.CreatedAt,
	); err != nil {
		return syncdomain.Event{}, err
	}
	event.Payload = []byte(payload)
	return event, nil
}

func scanAck(row rowScanner) (syncdomain.CursorAck, error) {
	var ack syncdomain.CursorAck
	var cursorEventID sql.NullString
	var cursorCreatedAt sql.NullTime
	if err := row.Scan(
		&ack.UserID,
		&ack.WorkspaceID,
		&ack.DeviceID,
		&cursorEventID,
		&cursorCreatedAt,
		&ack.AckedAt,
	); err != nil {
		return syncdomain.CursorAck{}, err
	}
	ack.CursorEventID = nullStringPtr(cursorEventID)
	if cursorCreatedAt.Valid {
		ack.CursorCreatedAt = &cursorCreatedAt.Time
	}
	return ack, nil
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
