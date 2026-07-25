package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	callsapp "github.com/duclamdev/application-chat/backend/internal/modules/calls/application"
	callsdomain "github.com/duclamdev/application-chat/backend/internal/modules/calls/domain"
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

func (r *Repository) WorkspaceZoneID(ctx context.Context, workspaceID string) (string, error) {
	var zoneID string
	err := r.pool.QueryRow(ctx, `
SELECT zone_id::text
FROM workspaces
WHERE id = $1::uuid
  AND status = 'active'
  AND deleted_at IS NULL
`, workspaceID).Scan(&zoneID)
	return zoneID, err
}

func (r *Repository) Create(ctx context.Context, params callsapp.CreateParams) (callsdomain.Call, error) {
	row := r.pool.QueryRow(ctx, `
INSERT INTO call_sessions (workspace_id, channel_id, initiator_user_id, target_user_id, client_call_id, mode, metadata)
SELECT c.workspace_id, c.id, $3::uuid, $4::uuid, NULLIF($5, ''), $6, $7::jsonb
FROM channels c
WHERE c.workspace_id = $1::uuid
  AND c.id = $2::uuid
  AND c.deleted_at IS NULL
  AND c.status = 'active'
  AND EXISTS (
      SELECT 1 FROM channel_members cm
      WHERE cm.channel_id = c.id AND cm.user_id = $3::uuid AND cm.status IN ('active', 'muted')
  )
  AND EXISTS (
      SELECT 1 FROM channel_members cm
      WHERE cm.channel_id = c.id AND cm.user_id = $4::uuid AND cm.status IN ('active', 'muted')
  )
ON CONFLICT (workspace_id, initiator_user_id, client_call_id)
WHERE client_call_id IS NOT NULL AND length(trim(client_call_id)) > 0
DO UPDATE SET updated_at = call_sessions.updated_at
RETURNING id::text, workspace_id::text, channel_id::text, initiator_user_id::text, target_user_id::text,
          client_call_id, mode, status, metadata::text, started_at, ended_at, created_at, updated_at
	`, params.WorkspaceID, params.ChannelID, params.InitiatorID, params.TargetUserID, params.ClientCallID, params.Mode, string(params.Metadata))
	call, err := scanCall(row)
	if errors.Is(err, callsdomain.ErrCallNotFound) {
		return callsdomain.Call{}, callsdomain.ErrCallParticipantDenied
	}
	return call, err
}

func (r *Repository) Get(ctx context.Context, workspaceID string, callID string) (callsdomain.Call, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id::text, workspace_id::text, channel_id::text, initiator_user_id::text, target_user_id::text,
       client_call_id, mode, status, metadata::text, started_at, ended_at, created_at, updated_at
FROM call_sessions
WHERE workspace_id = $1::uuid AND id = $2::uuid
`, workspaceID, callID)
	return scanCall(row)
}

func (r *Repository) UpdateStatus(ctx context.Context, params callsapp.StatusParams) (callsdomain.Call, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE call_sessions
SET status = $3,
    started_at = CASE WHEN $3 = 'accepted' THEN COALESCE(started_at, now()) ELSE started_at END,
    ended_at = CASE WHEN $3 IN ('rejected', 'cancelled', 'ended', 'missed') THEN COALESCE(ended_at, now()) ELSE ended_at END
WHERE workspace_id = $1::uuid
  AND id = $2::uuid
  AND (initiator_user_id = $4::uuid OR target_user_id = $4::uuid)
  AND status = $5
RETURNING id::text, workspace_id::text, channel_id::text, initiator_user_id::text, target_user_id::text,
          client_call_id, mode, status, metadata::text, started_at, ended_at, created_at, updated_at
`, params.WorkspaceID, params.CallID, params.Status, params.ActorUserID, params.ExpectedStatus)
	call, err := scanCall(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return callsdomain.Call{}, callsdomain.ErrCallInvalidTransition
	}
	return call, err
}

func (r *Repository) ExpireRingingCall(ctx context.Context, workspaceID string, callID string, before time.Time) (callsdomain.Call, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE call_sessions
SET status = 'missed',
    ended_at = COALESCE(ended_at, now())
WHERE workspace_id = $1::uuid
  AND id = $2::uuid
  AND status = 'ringing'
  AND created_at <= $3
RETURNING id::text, workspace_id::text, channel_id::text, initiator_user_id::text, target_user_id::text,
          client_call_id, mode, status, metadata::text, started_at, ended_at, created_at, updated_at
`, workspaceID, callID, before)
	call, err := scanCall(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return callsdomain.Call{}, callsdomain.ErrCallInvalidTransition
	}
	return call, err
}

func (r *Repository) ExpireRinging(ctx context.Context, before time.Time, limit int) ([]callsdomain.Call, error) {
	rows, err := r.pool.Query(ctx, `
WITH expired AS (
    SELECT id
    FROM call_sessions
    WHERE status = 'ringing'
      AND created_at <= $1
    ORDER BY created_at ASC
    LIMIT $2
    FOR UPDATE SKIP LOCKED
)
UPDATE call_sessions cs
SET status = 'missed',
    ended_at = COALESCE(cs.ended_at, now())
FROM expired
WHERE cs.id = expired.id
RETURNING cs.id::text, cs.workspace_id::text, cs.channel_id::text,
          cs.initiator_user_id::text, cs.target_user_id::text,
          cs.client_call_id, cs.mode, cs.status, cs.metadata::text,
          cs.started_at, cs.ended_at, cs.created_at, cs.updated_at
`, before, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	calls := make([]callsdomain.Call, 0)
	for rows.Next() {
		call, err := scanCall(rows)
		if err != nil {
			return nil, err
		}
		calls = append(calls, call)
	}
	return calls, rows.Err()
}

func (r *Repository) CreateSignal(ctx context.Context, params callsapp.SignalParams) (callsdomain.Signal, error) {
	row := r.pool.QueryRow(ctx, `
INSERT INTO call_signals (workspace_id, call_id, sender_user_id, signal_type, payload)
SELECT cs.workspace_id, cs.id, $3::uuid, $4, $5::jsonb
FROM call_sessions cs
WHERE cs.workspace_id = $1::uuid
  AND cs.id = $2::uuid
  AND (cs.initiator_user_id = $3::uuid OR cs.target_user_id = $3::uuid)
RETURNING id::text, workspace_id::text, call_id::text, sender_user_id::text, signal_type, payload::text, created_at
`, params.WorkspaceID, params.CallID, params.SenderUserID, params.SignalType, string(params.Payload))
	signal, err := scanSignal(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return callsdomain.Signal{}, callsdomain.ErrCallNotFound
	}
	return signal, err
}

func (r *Repository) CreateCallMessage(ctx context.Context, params callsapp.CallMessageParams) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	metadata, err := mergeMetadata(params.Metadata, map[string]any{
		"client_message_id": "call-" + params.CallID,
	})
	if err != nil {
		return err
	}
	var messageID string
	err = tx.QueryRow(ctx, `
INSERT INTO messages (workspace_id, channel_id, sender_id, kind, body, metadata)
SELECT $1::uuid, $2::uuid, $3::uuid, 'event', $4, $5::jsonb
WHERE NOT EXISTS (
    SELECT 1
    FROM messages
    WHERE workspace_id = $1::uuid
      AND channel_id = $2::uuid
      AND metadata ->> 'client_message_id' = $6
)
RETURNING id::text
`, params.WorkspaceID, params.ChannelID, params.SenderID, params.Body, string(metadata), "call-"+params.CallID).Scan(&messageID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := insertOutbox(ctx, tx, "message", messageID, "MessageCreated", map[string]any{
		"workspace_id":       params.WorkspaceID,
		"channel_id":         params.ChannelID,
		"message_id":         messageID,
		"sender_id":          params.SenderID,
		"mentioned_user_ids": []string{},
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type commandExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func insertOutbox(ctx context.Context, exec commandExecutor, aggregateType string, aggregateID string, eventType string, payload map[string]any) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = exec.Exec(ctx, `
INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload)
VALUES ($1, $2::uuid, $3, $4::jsonb)
`, aggregateType, aggregateID, eventType, string(payloadBytes))
	return err
}

func mergeMetadata(raw []byte, extra map[string]any) ([]byte, error) {
	metadata := map[string]any{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &metadata); err != nil {
			return nil, err
		}
	}
	for key, value := range extra {
		metadata[key] = value
	}
	return json.Marshal(metadata)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCall(row rowScanner) (callsdomain.Call, error) {
	var call callsdomain.Call
	var clientCallID sql.NullString
	var metadata string
	var startedAt sql.NullTime
	var endedAt sql.NullTime
	if err := row.Scan(
		&call.ID,
		&call.WorkspaceID,
		&call.ChannelID,
		&call.InitiatorUserID,
		&call.TargetUserID,
		&clientCallID,
		&call.Mode,
		&call.Status,
		&metadata,
		&startedAt,
		&endedAt,
		&call.CreatedAt,
		&call.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return callsdomain.Call{}, callsdomain.ErrCallNotFound
		}
		return callsdomain.Call{}, err
	}
	call.ClientCallID = nullStringPtr(clientCallID)
	call.Metadata = []byte(metadata)
	if startedAt.Valid {
		call.StartedAt = &startedAt.Time
	}
	if endedAt.Valid {
		call.EndedAt = &endedAt.Time
	}
	return call, nil
}

func scanSignal(row rowScanner) (callsdomain.Signal, error) {
	var signal callsdomain.Signal
	var payload string
	if err := row.Scan(
		&signal.ID,
		&signal.WorkspaceID,
		&signal.CallID,
		&signal.SenderUserID,
		&signal.SignalType,
		&payload,
		&signal.CreatedAt,
	); err != nil {
		return callsdomain.Signal{}, err
	}
	signal.Payload = []byte(payload)
	return signal, nil
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
