package postgres

import (
	"context"
	"time"

	adminapp "github.com/duclamdev/application-chat/backend/internal/modules/admin/application"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) DashboardStats(ctx context.Context, workspaceID string) (adminapp.DashboardStats, error) {
	var stats adminapp.DashboardStats
	err := r.pool.QueryRow(ctx, `
SELECT
    $1::text,
    (SELECT count(*) FROM workspace_members WHERE workspace_id = $1::uuid AND status = 'active'),
    (SELECT count(*) FROM channels WHERE workspace_id = $1::uuid AND deleted_at IS NULL),
    (SELECT count(*) FROM messages WHERE workspace_id = $1::uuid AND deleted_at IS NULL),
    (SELECT count(*) FROM files WHERE workspace_id = $1::uuid AND deleted_at IS NULL),
    (SELECT count(*) FROM bots WHERE workspace_id = $1::uuid AND deleted_at IS NULL),
    (SELECT count(*) FROM incoming_webhooks WHERE workspace_id = $1::uuid),
    (SELECT count(*) FROM outgoing_webhooks WHERE workspace_id = $1::uuid),
    (SELECT count(*) FROM audit_logs WHERE workspace_id = $1::uuid),
    (SELECT count(*) FROM backup_jobs WHERE workspace_id = $1::uuid OR workspace_id IS NULL)
`, workspaceID).Scan(
		&stats.WorkspaceID,
		&stats.ActiveMembers,
		&stats.Channels,
		&stats.Messages,
		&stats.Files,
		&stats.Bots,
		&stats.IncomingWebhooks,
		&stats.OutgoingWebhooks,
		&stats.AuditLogs,
		&stats.BackupJobs,
	)
	if err != nil {
		return adminapp.DashboardStats{}, err
	}
	if err := r.pool.QueryRow(ctx, `
SELECT COALESCE(sum(byte_size), 0)
FROM files
WHERE workspace_id = $1::uuid AND deleted_at IS NULL
`, workspaceID).Scan(&stats.StorageBytes); err != nil {
		return adminapp.DashboardStats{}, err
	}

	activityRows, err := r.pool.Query(ctx, `
WITH days AS (
    SELECT generate_series(current_date - interval '6 days', current_date, interval '1 day')::date AS day
)
SELECT to_char(days.day, 'YYYY-MM-DD'),
       count(m.id),
       count(DISTINCT m.sender_id)
FROM days
LEFT JOIN messages m
  ON m.workspace_id = $1::uuid
 AND m.deleted_at IS NULL
 AND m.created_at >= days.day
 AND m.created_at < days.day + interval '1 day'
GROUP BY days.day
ORDER BY days.day
`, workspaceID)
	if err != nil {
		return adminapp.DashboardStats{}, err
	}
	defer activityRows.Close()
	for activityRows.Next() {
		var point adminapp.ActivityPoint
		if err := activityRows.Scan(&point.Date, &point.Messages, &point.Users); err != nil {
			return adminapp.DashboardStats{}, err
		}
		stats.Activity = append(stats.Activity, point)
	}
	if err := activityRows.Err(); err != nil {
		return adminapp.DashboardStats{}, err
	}

	rankRows, err := r.pool.Query(ctx, `
SELECT c.id::text, c.name, count(m.id) AS messages_count
FROM channels c
LEFT JOIN messages m
  ON m.workspace_id = c.workspace_id
 AND m.channel_id = c.id
 AND m.deleted_at IS NULL
WHERE c.workspace_id = $1::uuid AND c.deleted_at IS NULL
GROUP BY c.id, c.name
ORDER BY messages_count DESC, c.name
LIMIT 5
`, workspaceID)
	if err != nil {
		return adminapp.DashboardStats{}, err
	}
	defer rankRows.Close()
	for rankRows.Next() {
		var rank adminapp.ChannelRank
		if err := rankRows.Scan(&rank.ChannelID, &rank.Name, &rank.MessagesCount); err != nil {
			return adminapp.DashboardStats{}, err
		}
		stats.ChannelRanks = append(stats.ChannelRanks, rank)
	}
	if err := rankRows.Err(); err != nil {
		return adminapp.DashboardStats{}, err
	}
	return stats, nil
}

func (r *Repository) ListChannels(ctx context.Context, workspaceID string) ([]adminapp.ChannelOverview, error) {
	rows, err := r.pool.Query(ctx, `
SELECT c.id::text,
       c.name,
       COALESCE(c.slug::text, ''),
       c.type,
       c.status,
       count(DISTINCT cm.user_id) FILTER (WHERE cm.status IN ('active', 'muted')),
       count(DISTINCT m.id) FILTER (WHERE m.deleted_at IS NULL),
       COALESCE(c.settings->>'bot_session_mode', '') = 'private',
       c.updated_at
FROM channels c
LEFT JOIN channel_members cm ON cm.channel_id = c.id
LEFT JOIN messages m ON m.workspace_id = c.workspace_id AND m.channel_id = c.id
WHERE c.workspace_id = $1::uuid AND c.deleted_at IS NULL
GROUP BY c.id
ORDER BY c.type, c.name
`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	channels := make([]adminapp.ChannelOverview, 0)
	for rows.Next() {
		var channel adminapp.ChannelOverview
		var updatedAt time.Time
		if err := rows.Scan(&channel.ID, &channel.Name, &channel.Slug, &channel.Type, &channel.Status, &channel.MemberCount, &channel.MessageCount, &channel.PrivateSessionMode, &updatedAt); err != nil {
			return nil, err
		}
		channel.UpdatedAt = updatedAt.Format("2006-01-02T15:04:05Z07:00")
		channels = append(channels, channel)
	}
	return channels, rows.Err()
}

func (r *Repository) ListRecentMessages(ctx context.Context, workspaceID string, limit int) ([]adminapp.MessageOverview, error) {
	rows, err := r.pool.Query(ctx, `
SELECT m.id::text,
       m.channel_id::text,
       c.name,
       COALESCE(NULLIF(u.display_name, ''), NULLIF(u.username::text, ''), u.email::text, b.name, 'Hệ thống'),
       m.kind,
       CASE
           WHEN c.settings @> '{"bot_session":true}'::jsonb THEN '[Nội dung phiên riêng tư]'
           ELSE m.body
       END,
       m.created_at
FROM messages m
JOIN channels c ON c.id = m.channel_id AND c.deleted_at IS NULL
LEFT JOIN users u ON u.id = m.sender_id
LEFT JOIN bots b ON b.id::text = m.metadata->>'bot_id'
WHERE m.workspace_id = $1::uuid AND m.deleted_at IS NULL
ORDER BY m.created_at DESC, m.id DESC
LIMIT $2
`, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := make([]adminapp.MessageOverview, 0)
	for rows.Next() {
		var message adminapp.MessageOverview
		var createdAt time.Time
		if err := rows.Scan(&message.ID, &message.ChannelID, &message.ChannelName, &message.SenderName, &message.Kind, &message.Body, &createdAt); err != nil {
			return nil, err
		}
		message.CreatedAt = createdAt.Format("2006-01-02T15:04:05Z07:00")
		messages = append(messages, message)
	}
	return messages, rows.Err()
}
