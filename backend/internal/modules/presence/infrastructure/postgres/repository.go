package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	presenceapp "github.com/duclamdev/application-chat/backend/internal/modules/presence/application"
	presencedomain "github.com/duclamdev/application-chat/backend/internal/modules/presence/domain"
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

func (r *Repository) Upsert(ctx context.Context, params presenceapp.UpsertParams) (presencedomain.Presence, error) {
	if params.Status == "offline" {
		return r.markSocketOffline(ctx, params)
	}

	row := r.pool.QueryRow(ctx, `
INSERT INTO user_presence (user_id, workspace_id, device_id, socket_id, node_id, status, last_heartbeat_at, connected_at, metadata)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, now(), now(), $7::jsonb)
ON CONFLICT (user_id, device_id)
DO UPDATE SET workspace_id = EXCLUDED.workspace_id,
              socket_id = EXCLUDED.socket_id,
              node_id = EXCLUDED.node_id,
              status = EXCLUDED.status,
              last_heartbeat_at = now(),
              metadata = EXCLUDED.metadata
RETURNING user_id::text, workspace_id::text, device_id, socket_id, node_id, status,
          last_heartbeat_at, connected_at, metadata::text
`, params.UserID, params.WorkspaceID, params.DeviceID, params.SocketID, params.NodeID, params.Status, string(params.Metadata))
	presence, err := scanPresence(row)
	if isUniqueViolation(err) {
		row = r.pool.QueryRow(ctx, `
UPDATE user_presence
SET user_id = $1::uuid,
    workspace_id = $2::uuid,
    device_id = $3,
    node_id = $5,
    status = $6,
    last_heartbeat_at = now(),
    metadata = $7::jsonb
WHERE socket_id = $4
RETURNING user_id::text, workspace_id::text, device_id, socket_id, node_id, status,
          last_heartbeat_at, connected_at, metadata::text
`, params.UserID, params.WorkspaceID, params.DeviceID, params.SocketID, params.NodeID, params.Status, string(params.Metadata))
		return scanPresence(row)
	}
	return presence, err
}

func (r *Repository) markSocketOffline(ctx context.Context, params presenceapp.UpsertParams) (presencedomain.Presence, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE user_presence
SET status = 'offline',
    node_id = $5,
    last_heartbeat_at = now(),
    metadata = $6::jsonb
WHERE user_id = $1::uuid
  AND workspace_id = $2::uuid
  AND device_id = $3
  AND socket_id = $4
RETURNING user_id::text, workspace_id::text, device_id, socket_id, node_id, status,
          last_heartbeat_at, connected_at, metadata::text
`, params.UserID, params.WorkspaceID, params.DeviceID, params.SocketID, params.NodeID, string(params.Metadata))
	presence, err := scanPresence(row)
	if !errors.Is(err, pgx.ErrNoRows) {
		return presence, err
	}

	row = r.pool.QueryRow(ctx, `
SELECT user_id::text, workspace_id::text, device_id, socket_id, node_id, status,
       last_heartbeat_at, connected_at, metadata::text
FROM user_presence
WHERE user_id = $1::uuid
  AND workspace_id = $2::uuid
  AND device_id = $3
`, params.UserID, params.WorkspaceID, params.DeviceID)
	return scanPresence(row)
}

func (r *Repository) List(ctx context.Context, workspaceID string, limit int) ([]presencedomain.Presence, error) {
	rows, err := r.pool.Query(ctx, `
SELECT user_id::text, workspace_id::text, device_id, socket_id, node_id, status,
       last_heartbeat_at, connected_at, metadata::text
FROM user_presence
WHERE workspace_id = $1::uuid
ORDER BY CASE status WHEN 'online' THEN 1 WHEN 'away' THEN 2 ELSE 3 END,
         last_heartbeat_at DESC
LIMIT $2
`, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	presences := make([]presencedomain.Presence, 0)
	for rows.Next() {
		presence, err := scanPresence(rows)
		if err != nil {
			return nil, err
		}
		presences = append(presences, presence)
	}
	return presences, rows.Err()
}

func (r *Repository) MarkOfflineStale(ctx context.Context, staleAfter time.Duration) (int, error) {
	command, err := r.pool.Exec(ctx, `
UPDATE user_presence
SET status = 'offline'
WHERE status IN ('online', 'away')
  AND last_heartbeat_at < now() - make_interval(secs => $1::int)
`, int(staleAfter.Seconds()))
	if err != nil {
		return 0, err
	}
	return int(command.RowsAffected()), nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanPresence(row rowScanner) (presencedomain.Presence, error) {
	var presence presencedomain.Presence
	var workspaceID sql.NullString
	var metadata string
	if err := row.Scan(
		&presence.UserID,
		&workspaceID,
		&presence.DeviceID,
		&presence.SocketID,
		&presence.NodeID,
		&presence.Status,
		&presence.LastHeartbeatAt,
		&presence.ConnectedAt,
		&metadata,
	); err != nil {
		return presencedomain.Presence{}, err
	}
	presence.WorkspaceID = nullStringPtr(workspaceID)
	presence.Metadata = []byte(metadata)
	return presence, nil
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
