package postgres

import (
	"context"
	"database/sql"
	"time"

	auditapp "github.com/duclamdev/application-chat/backend/internal/modules/audit/application"
	auditdomain "github.com/duclamdev/application-chat/backend/internal/modules/audit/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) List(ctx context.Context, params auditapp.ListParams) ([]auditdomain.Log, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, workspace_id::text, actor_user_id::text, action, entity_type, entity_id::text,
       ip_address::text, user_agent, before_data::text, after_data::text, metadata::text, created_at
FROM audit_logs
WHERE workspace_id = $1::uuid
  AND ($2 = '' OR actor_user_id = NULLIF($2, '')::uuid)
  AND ($3 = '' OR action = $3)
  AND ($4 = '' OR entity_type = $4)
  AND ($5 = '' OR entity_id = NULLIF($5, '')::uuid)
  AND ($6::timestamptz IS NULL OR created_at >= $6)
  AND ($7::timestamptz IS NULL OR created_at <= $7)
ORDER BY created_at DESC
LIMIT $8
`, params.WorkspaceID, params.FilterActorID, params.Action, params.EntityType, params.EntityID, params.From, params.To, params.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	logs := make([]auditdomain.Log, 0)
	for rows.Next() {
		log, err := scanLog(rows)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanLog(row rowScanner) (auditdomain.Log, error) {
	var log auditdomain.Log
	var workspaceID sql.NullString
	var actorUserID sql.NullString
	var entityID sql.NullString
	var ipAddress sql.NullString
	var userAgent sql.NullString
	var beforeData sql.NullString
	var afterData sql.NullString
	var metadata sql.NullString
	var createdAt time.Time
	if err := row.Scan(
		&log.ID,
		&workspaceID,
		&actorUserID,
		&log.Action,
		&log.EntityType,
		&entityID,
		&ipAddress,
		&userAgent,
		&beforeData,
		&afterData,
		&metadata,
		&createdAt,
	); err != nil {
		return auditdomain.Log{}, err
	}
	log.WorkspaceID = nullStringPtr(workspaceID)
	log.ActorUserID = nullStringPtr(actorUserID)
	log.EntityID = nullStringPtr(entityID)
	log.IPAddress = nullStringPtr(ipAddress)
	log.UserAgent = nullStringPtr(userAgent)
	log.BeforeData = nullStringBytes(beforeData)
	log.AfterData = nullStringBytes(afterData)
	log.Metadata = nullStringBytes(metadata)
	log.CreatedAt = createdAt
	return log, nil
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func nullStringBytes(value sql.NullString) []byte {
	if !value.Valid {
		return nil
	}
	return []byte(value.String)
}
