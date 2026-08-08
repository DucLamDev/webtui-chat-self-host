package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	moderationapp "github.com/duclamdev/application-chat/backend/internal/modules/moderation/application"
	moderationdomain "github.com/duclamdev/application-chat/backend/internal/modules/moderation/domain"
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

func (r *Repository) IsActiveWorkspaceMember(ctx context.Context, workspaceID string, userID string) (bool, error) {
	var member bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM workspace_members member
    JOIN users user_account
      ON user_account.id = member.user_id
     AND user_account.status = 'active'
     AND user_account.deleted_at IS NULL
    WHERE member.workspace_id = $1::uuid
      AND member.user_id = $2::uuid
      AND member.status = 'active'
)
`, workspaceID, userID).Scan(&member)
	return member, err
}

func (r *Repository) ResolveReportTarget(
	ctx context.Context,
	workspaceID string,
	reporterUserID string,
	targetType string,
	targetID string,
) (moderationdomain.ReportTarget, error) {
	var target moderationdomain.ReportTarget
	var err error
	switch targetType {
	case "user":
		var snapshot string
		err = r.pool.QueryRow(ctx, `
SELECT member.user_id::text,
       jsonb_build_object(
           'user_id', user_account.id::text,
           'username', user_account.username::text,
           'display_name', user_account.display_name,
           'created_at', user_account.created_at
       )::text
FROM workspace_members member
JOIN users user_account
  ON user_account.id = member.user_id
 AND user_account.status = 'active'
 AND user_account.deleted_at IS NULL
WHERE member.workspace_id = $1::uuid
  AND member.user_id = $2::uuid
  AND member.status = 'active'
`, workspaceID, targetID).Scan(&target.UserID, &snapshot)
		target.Snapshot = json.RawMessage(snapshot)
	case "message":
		var snapshot string
		err = r.pool.QueryRow(ctx, `
SELECT COALESCE(message.sender_id::text, ''),
       jsonb_strip_nulls(jsonb_build_object(
           'message_id', message.id::text,
           'channel_id', message.channel_id::text,
           'sender_user_id', message.sender_id::text,
           'sender_display_name', sender.display_name,
           'producer_kind', CASE
               WHEN message.sender_id IS NOT NULL THEN 'user'
               WHEN message.kind = 'bot' THEN 'bot'
               WHEN message.kind = 'event' THEN 'integration'
               ELSE 'system'
           END,
           'kind', message.kind,
           'body_excerpt', left(message.body, 2000),
           'body_sha256', encode(digest(message.body, 'sha256'), 'hex'),
           'created_at', message.created_at
       ))::text
FROM messages message
JOIN channel_members reporter_channel
  ON reporter_channel.channel_id = message.channel_id
 AND reporter_channel.user_id = $2::uuid
 AND reporter_channel.status IN ('active', 'muted')
LEFT JOIN users sender
  ON sender.id = message.sender_id
WHERE message.workspace_id = $1::uuid
  AND message.id = $3::uuid
  AND message.kind IN ('text', 'file', 'bot', 'event')
  AND message.deleted_at IS NULL
`, workspaceID, reporterUserID, targetID).Scan(&target.UserID, &snapshot)
		target.Snapshot = json.RawMessage(snapshot)
	default:
		return moderationdomain.ReportTarget{}, moderationdomain.ErrReportTarget
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return moderationdomain.ReportTarget{}, moderationdomain.ErrReportTarget
	}
	return target, err
}

func (r *Repository) CreateReport(ctx context.Context, params moderationapp.CreateReportParams) (moderationdomain.Report, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return moderationdomain.Report{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	// Serialize the rolling-window check for one reporter. Without this lock,
	// concurrent reports for different targets can all observe the same count
	// and exceed the hard abuse limit.
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, params.ReporterUserID); err != nil {
		return moderationdomain.Report{}, err
	}
	var count int
	if err := tx.QueryRow(ctx, `
SELECT count(*)
FROM moderation_reports
WHERE reporter_user_id = $1::uuid
  AND created_at >= $2
`, params.ReporterUserID, params.RateLimitSince).Scan(&count); err != nil {
		return moderationdomain.Report{}, err
	}
	if count >= params.MaxReports {
		return moderationdomain.Report{}, moderationdomain.ErrReportRateLimit
	}

	var reportID string
	var createdAt, updatedAt time.Time
	err = tx.QueryRow(ctx, `
INSERT INTO moderation_reports (
    workspace_id, reporter_user_id, target_type, target_id, target_user_id,
    target_snapshot, reason, details
)
VALUES (
    $1::uuid, $2::uuid, $3, $4::uuid, NULLIF($5, '')::uuid,
    $6::jsonb, $7, NULLIF($8, '')
)
RETURNING id::text, created_at, updated_at
`, params.WorkspaceID, params.ReporterUserID, params.TargetType, params.TargetID,
		params.TargetUserID, string(params.TargetSnapshot), params.Reason, params.Details).Scan(
		&reportID,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return moderationdomain.Report{}, moderationdomain.ErrReportDuplicate
		}
		return moderationdomain.Report{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return moderationdomain.Report{}, err
	}
	reporterUserID := params.ReporterUserID
	var targetUserID *string
	if strings.TrimSpace(params.TargetUserID) != "" {
		value := params.TargetUserID
		targetUserID = &value
	}
	var details *string
	if strings.TrimSpace(params.Details) != "" {
		value := params.Details
		details = &value
	}
	return moderationdomain.Report{
		ID:             reportID,
		WorkspaceID:    params.WorkspaceID,
		ReporterUserID: &reporterUserID,
		TargetType:     params.TargetType,
		TargetID:       params.TargetID,
		TargetUserID:   targetUserID,
		TargetSnapshot: append(json.RawMessage(nil), params.TargetSnapshot...),
		Reason:         params.Reason,
		Details:        details,
		Status:         "pending",
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
	}, nil
}

func (r *Repository) ListReports(ctx context.Context, params moderationapp.ListReportsParams) ([]moderationdomain.Report, error) {
	rows, err := r.pool.Query(ctx, reportSelect+`
WHERE report.workspace_id = $1::uuid
  AND (NULLIF($2, '') IS NULL OR report.status = $2)
  AND (NULLIF($3, '') IS NULL OR report.target_type = $3)
ORDER BY report.created_at DESC, report.id DESC
LIMIT $4 OFFSET $5
`, params.WorkspaceID, params.Status, params.TargetType, params.Limit, params.Offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reports := make([]moderationdomain.Report, 0)
	for rows.Next() {
		report, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, rows.Err()
}

func (r *Repository) UpdateReport(ctx context.Context, params moderationapp.UpdateReportParams) (moderationdomain.Report, error) {
	var reportID string
	err := r.pool.QueryRow(ctx, `
UPDATE moderation_reports
SET status = $3,
    resolution_note = NULLIF($4, ''),
    resolved_by = CASE WHEN $3 IN ('resolved', 'dismissed') THEN $5::uuid ELSE NULL END,
    resolved_at = CASE WHEN $3 IN ('resolved', 'dismissed') THEN now() ELSE NULL END
WHERE workspace_id = $1::uuid
  AND id = $2::uuid
RETURNING id::text
`, params.WorkspaceID, params.ReportID, params.Status, params.Resolution, params.ResolvedBy).Scan(&reportID)
	if errors.Is(err, pgx.ErrNoRows) {
		return moderationdomain.Report{}, moderationdomain.ErrReportNotFound
	}
	if err != nil {
		return moderationdomain.Report{}, err
	}
	return r.findReport(ctx, params.WorkspaceID, reportID)
}

func (r *Repository) CreateBlock(ctx context.Context, params moderationapp.CreateBlockParams) (moderationdomain.UserBlock, error) {
	var blockID string
	err := r.pool.QueryRow(ctx, `
INSERT INTO user_blocks (workspace_id, blocker_user_id, blocked_user_id, reason)
VALUES ($1::uuid, $2::uuid, $3::uuid, NULLIF($4, ''))
ON CONFLICT (workspace_id, blocker_user_id, blocked_user_id)
DO UPDATE SET
    reason = COALESCE(NULLIF(EXCLUDED.reason, ''), user_blocks.reason),
    updated_at = now()
RETURNING id::text
`, params.WorkspaceID, params.BlockerUserID, params.BlockedUserID, params.Reason).Scan(&blockID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23514" {
			return moderationdomain.UserBlock{}, moderationdomain.ErrInvalidBlockPair
		}
		return moderationdomain.UserBlock{}, err
	}
	return r.findBlock(ctx, params.WorkspaceID, params.BlockerUserID, blockID)
}

func (r *Repository) DeleteBlock(ctx context.Context, workspaceID string, blockerUserID string, blockedUserID string) error {
	_, err := r.pool.Exec(ctx, `
DELETE FROM user_blocks
WHERE workspace_id = $1::uuid
  AND blocker_user_id = $2::uuid
  AND blocked_user_id = $3::uuid
`, workspaceID, blockerUserID, blockedUserID)
	return err
}

func (r *Repository) ListBlocks(ctx context.Context, workspaceID string, blockerUserID string) ([]moderationdomain.UserBlock, error) {
	rows, err := r.pool.Query(ctx, blockSelect+`
WHERE block.workspace_id = $1::uuid
  AND block.blocker_user_id = $2::uuid
ORDER BY block.created_at DESC, block.id DESC
`, workspaceID, blockerUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	blocks := make([]moderationdomain.UserBlock, 0)
	for rows.Next() {
		block, err := scanBlock(rows)
		if err != nil {
			return nil, err
		}
		blocks = append(blocks, block)
	}
	return blocks, rows.Err()
}

func (r *Repository) IsInteractionBlocked(ctx context.Context, workspaceID string, firstUserID string, secondUserID string) (bool, error) {
	if strings.TrimSpace(firstUserID) == "" || strings.TrimSpace(secondUserID) == "" || firstUserID == secondUserID {
		return false, nil
	}
	var blocked bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM user_blocks block
    WHERE block.workspace_id = $1::uuid
      AND (
          (block.blocker_user_id = $2::uuid AND block.blocked_user_id = $3::uuid) OR
          (block.blocker_user_id = $3::uuid AND block.blocked_user_id = $2::uuid)
      )
)
`, workspaceID, firstUserID, secondUserID).Scan(&blocked)
	return blocked, err
}

func (r *Repository) IsDirectChannelBlocked(ctx context.Context, workspaceID string, channelID string, actorUserID string) (bool, error) {
	var blocked bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM channels channel
    JOIN direct_conversations direct
      ON direct.channel_id = channel.id
     AND direct.workspace_id = channel.workspace_id
     AND direct.archived_at IS NULL
    JOIN direct_conversation_members other_member
      ON other_member.direct_conversation_id = direct.id
     AND other_member.user_id <> $3::uuid
    JOIN user_blocks block
      ON block.workspace_id = channel.workspace_id
     AND (
          (block.blocker_user_id = $3::uuid AND block.blocked_user_id = other_member.user_id) OR
          (block.blocked_user_id = $3::uuid AND block.blocker_user_id = other_member.user_id)
     )
    WHERE channel.workspace_id = $1::uuid
      AND channel.id = $2::uuid
      AND channel.type = 'direct'
      AND channel.deleted_at IS NULL
)
`, workspaceID, channelID, actorUserID).Scan(&blocked)
	return blocked, err
}

func (r *Repository) RecordAudit(ctx context.Context, event moderationapp.AuditEvent) error {
	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	payload, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
INSERT INTO audit_logs (workspace_id, actor_user_id, action, entity_type, entity_id, metadata)
VALUES ($1::uuid, $2::uuid, $3, $4, NULLIF($5, '')::uuid, $6::jsonb)
`, event.WorkspaceID, event.ActorUserID, event.Action, event.EntityType, event.EntityID, string(payload))
	return err
}

func (r *Repository) findReport(ctx context.Context, workspaceID string, reportID string) (moderationdomain.Report, error) {
	report, err := scanReport(r.pool.QueryRow(ctx, reportSelect+`
WHERE report.workspace_id = $1::uuid
  AND report.id = $2::uuid
`, workspaceID, reportID))
	if errors.Is(err, pgx.ErrNoRows) {
		return moderationdomain.Report{}, moderationdomain.ErrReportNotFound
	}
	return report, err
}

func (r *Repository) findBlock(ctx context.Context, workspaceID string, blockerUserID string, blockID string) (moderationdomain.UserBlock, error) {
	block, err := scanBlock(r.pool.QueryRow(ctx, blockSelect+`
WHERE block.workspace_id = $1::uuid
  AND block.blocker_user_id = $2::uuid
  AND block.id = $3::uuid
`, workspaceID, blockerUserID, blockID))
	if errors.Is(err, pgx.ErrNoRows) {
		return moderationdomain.UserBlock{}, moderationdomain.ErrBlockNotFound
	}
	return block, err
}

const reportSelect = `
SELECT
    report.id::text,
    report.workspace_id::text,
    report.reporter_user_id::text,
    reporter.display_name,
    report.target_type,
    report.target_id::text,
    report.target_user_id::text,
    target_user.display_name,
    report.target_snapshot::text,
    report.reason,
    report.details,
    report.status,
    report.resolution_note,
    report.resolved_by::text,
    report.resolved_at,
    report.created_at,
    report.updated_at
FROM moderation_reports report
LEFT JOIN users reporter ON reporter.id = report.reporter_user_id
LEFT JOIN users target_user ON target_user.id = report.target_user_id
`

const blockSelect = `
SELECT
    block.id::text,
    block.workspace_id::text,
    block.blocker_user_id::text,
    block.blocked_user_id::text,
    blocked_user.username::text,
    blocked_user.display_name,
    blocked_user.avatar_url,
    block.reason,
    block.created_at,
    block.updated_at
FROM user_blocks block
JOIN users blocked_user ON blocked_user.id = block.blocked_user_id
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanReport(row rowScanner) (moderationdomain.Report, error) {
	var report moderationdomain.Report
	var reporterUserID, reporterName, targetUserID, targetName sql.NullString
	var details, resolution, resolvedBy sql.NullString
	var targetSnapshot string
	var resolvedAt sql.NullTime
	err := row.Scan(
		&report.ID, &report.WorkspaceID,
		&reporterUserID, &reporterName,
		&report.TargetType, &report.TargetID,
		&targetUserID, &targetName,
		&targetSnapshot,
		&report.Reason, &details, &report.Status, &resolution, &resolvedBy, &resolvedAt,
		&report.CreatedAt, &report.UpdatedAt,
	)
	if err != nil {
		return moderationdomain.Report{}, err
	}
	report.ReporterUserID = nullStringPtr(reporterUserID)
	report.ReporterDisplayName = nullStringPtr(reporterName)
	report.TargetUserID = nullStringPtr(targetUserID)
	report.TargetUserDisplayName = nullStringPtr(targetName)
	report.TargetSnapshot = json.RawMessage(targetSnapshot)
	report.Details = nullStringPtr(details)
	report.ResolutionNote = nullStringPtr(resolution)
	report.ResolvedBy = nullStringPtr(resolvedBy)
	report.ResolvedAt = nullTimePtr(resolvedAt)
	return report, nil
}

func scanBlock(row rowScanner) (moderationdomain.UserBlock, error) {
	var block moderationdomain.UserBlock
	var avatar, reason sql.NullString
	err := row.Scan(
		&block.ID, &block.WorkspaceID, &block.BlockerUserID, &block.BlockedUserID,
		&block.BlockedUsername, &block.BlockedDisplayName, &avatar, &reason,
		&block.CreatedAt, &block.UpdatedAt,
	)
	if err != nil {
		return moderationdomain.UserBlock{}, err
	}
	block.BlockedAvatarURL = nullStringPtr(avatar)
	block.Reason = nullStringPtr(reason)
	return block, nil
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
