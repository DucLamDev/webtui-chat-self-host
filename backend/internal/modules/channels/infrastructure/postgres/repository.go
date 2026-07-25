package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	channelsapp "github.com/duclamdev/application-chat/backend/internal/modules/channels/application"
	channelsdomain "github.com/duclamdev/application-chat/backend/internal/modules/channels/domain"
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

func (r *Repository) CreateChannel(ctx context.Context, params channelsapp.CreateChannelParams) (channelsdomain.Channel, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return channelsdomain.Channel{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	row := tx.QueryRow(ctx, `
INSERT INTO channels (workspace_id, department_id, slug, name, description, type, created_by)
VALUES ($1::uuid, NULLIF($2, '')::uuid, $3, $4, NULLIF($5, ''), $6, $7::uuid)
RETURNING id::text, workspace_id::text, department_id::text, slug::text, name, description, type, status, created_by::text, created_at, updated_at, archived_at,
          COALESCE(settings->>'bot_session_mode', '') = 'private'
`, params.WorkspaceID, params.DepartmentID, params.Slug, params.Name, params.Description, params.Type, params.CreatedBy)
	channel, err := scanChannel(row)
	if err != nil {
		if isUniqueViolation(err) {
			return channelsdomain.Channel{}, channelsdomain.ErrChannelConflict
		}
		return channelsdomain.Channel{}, err
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO channel_members (channel_id, user_id, status)
VALUES ($1::uuid, $2::uuid, 'active')
ON CONFLICT (channel_id, user_id)
DO UPDATE SET status = 'active'
`, channel.ID, params.CreatedBy); err != nil {
		return channelsdomain.Channel{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return channelsdomain.Channel{}, err
	}
	return channel, nil
}

func (r *Repository) FindChannel(ctx context.Context, workspaceID string, channelID string) (channelsdomain.Channel, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id::text, workspace_id::text, department_id::text, slug::text, name, description, type, status, created_by::text, created_at, updated_at, archived_at,
       COALESCE(settings->>'bot_session_mode', '') = 'private'
FROM channels
WHERE workspace_id = $1::uuid AND id = $2::uuid AND deleted_at IS NULL
`, workspaceID, channelID)
	return scanChannel(row)
}

func (r *Repository) ListChannels(ctx context.Context, workspaceID string) ([]channelsdomain.Channel, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, workspace_id::text, department_id::text, slug::text, name, description, type, status, created_by::text, created_at, updated_at, archived_at,
       COALESCE(settings->>'bot_session_mode', '') = 'private',
       (SELECT count(*)::int FROM channel_members cm JOIN users u ON u.id = cm.user_id AND u.deleted_at IS NULL WHERE cm.channel_id = channels.id AND cm.status IN ('active', 'muted'))
FROM channels
WHERE workspace_id = $1::uuid AND deleted_at IS NULL AND type <> 'direct'
ORDER BY type, name
`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []channelsdomain.Channel
	for rows.Next() {
		channel, err := scanChannelWithMemberCount(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	return channels, rows.Err()
}

func scanChannelWithMemberCount(row rowScanner) (channelsdomain.Channel, error) {
	var channel channelsdomain.Channel
	var departmentID sql.NullString
	var slug sql.NullString
	var description sql.NullString
	var createdBy sql.NullString
	var archivedAt sql.NullTime
	if err := row.Scan(
		&channel.ID,
		&channel.WorkspaceID,
		&departmentID,
		&slug,
		&channel.Name,
		&description,
		&channel.Type,
		&channel.Status,
		&createdBy,
		&channel.CreatedAt,
		&channel.UpdatedAt,
		&archivedAt,
		&channel.PrivateSessionMode,
		&channel.MemberCount,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return channelsdomain.Channel{}, channelsdomain.ErrChannelNotFound
		}
		return channelsdomain.Channel{}, err
	}
	channel.DepartmentID = nullStringPtr(departmentID)
	channel.Slug = nullStringPtr(slug)
	channel.Description = nullStringPtr(description)
	channel.CreatedBy = nullStringPtr(createdBy)
	channel.ArchivedAt = nullTimePtr(archivedAt)
	return channel, nil
}

func (r *Repository) UpdateChannel(ctx context.Context, params channelsapp.UpdateChannelParams) (channelsdomain.Channel, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE channels
SET department_id = CASE WHEN $3::text IS NULL THEN department_id ELSE NULLIF($3::text, '')::uuid END,
    name = COALESCE($4, name),
    description = COALESCE($5, description)
WHERE workspace_id = $1::uuid AND id = $2::uuid AND deleted_at IS NULL
RETURNING id::text, workspace_id::text, department_id::text, slug::text, name, description, type, status, created_by::text, created_at, updated_at, archived_at,
          COALESCE(settings->>'bot_session_mode', '') = 'private'
`, params.WorkspaceID, params.ChannelID, params.DepartmentID, params.Name, params.Description)
	return scanChannel(row)
}

func (r *Repository) ArchiveChannel(ctx context.Context, workspaceID string, channelID string) error {
	command, err := r.pool.Exec(ctx, `
UPDATE channels
SET status = 'archived', archived_at = now()
WHERE workspace_id = $1::uuid AND id = $2::uuid AND deleted_at IS NULL
`, workspaceID, channelID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return channelsdomain.ErrChannelNotFound
	}
	return nil
}

func (r *Repository) CountMembers(ctx context.Context, workspaceID string, channelID string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
SELECT count(*)::int
FROM channel_members cm
JOIN channels c ON c.id = cm.channel_id AND c.deleted_at IS NULL
JOIN users u ON u.id = cm.user_id AND u.deleted_at IS NULL
WHERE c.workspace_id = $1::uuid
  AND c.id = $2::uuid
  AND cm.status IN ('active', 'muted')
`, workspaceID, channelID).Scan(&count)
	return count, err
}

func (r *Repository) ListMembers(ctx context.Context, workspaceID string, channelID string) ([]channelsdomain.Member, error) {
	rows, err := r.pool.Query(ctx, `
SELECT cm.channel_id::text, cm.user_id::text, u.email::text, u.username::text, u.display_name, u.avatar_url,
       cm.status, cm.last_read_at, cm.last_read_message_id::text, cm.joined_at, cm.created_at, cm.updated_at
FROM channel_members cm
JOIN channels c ON c.id = cm.channel_id AND c.deleted_at IS NULL
JOIN users u ON u.id = cm.user_id AND u.deleted_at IS NULL
WHERE c.workspace_id = $1::uuid AND c.id = $2::uuid
ORDER BY u.display_name
`, workspaceID, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []channelsdomain.Member
	for rows.Next() {
		member, err := scanMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func (r *Repository) FindMember(ctx context.Context, workspaceID string, channelID string, userID string) (channelsdomain.Member, error) {
	return r.member(ctx, workspaceID, channelID, userID)
}

func (r *Repository) AddMember(ctx context.Context, params channelsapp.AddMemberParams) (channelsdomain.Member, error) {
	command, err := r.pool.Exec(ctx, `
INSERT INTO channel_members (channel_id, user_id, status)
SELECT c.id, $3::uuid, 'active'
FROM channels c
JOIN workspace_members wm
  ON wm.workspace_id = c.workspace_id AND wm.user_id = $3::uuid AND wm.status = 'active'
WHERE c.workspace_id = $1::uuid AND c.id = $2::uuid AND c.deleted_at IS NULL
ON CONFLICT (channel_id, user_id)
DO UPDATE SET status = 'active'
`, params.WorkspaceID, params.ChannelID, params.UserID)
	if err != nil {
		return channelsdomain.Member{}, err
	}
	if command.RowsAffected() == 0 {
		return channelsdomain.Member{}, channelsdomain.ErrMemberNotFound
	}
	return r.member(ctx, params.WorkspaceID, params.ChannelID, params.UserID)
}

func (r *Repository) RequestJoin(ctx context.Context, params channelsapp.AddMemberParams) (channelsdomain.Member, error) {
	command, err := r.pool.Exec(ctx, `
INSERT INTO channel_members (channel_id, user_id, status)
SELECT c.id, $3::uuid, 'invited'
FROM channels c
JOIN workspace_members wm
  ON wm.workspace_id = c.workspace_id AND wm.user_id = $3::uuid AND wm.status = 'active'
WHERE c.workspace_id = $1::uuid AND c.id = $2::uuid AND c.deleted_at IS NULL AND c.type <> 'direct'
ON CONFLICT (channel_id, user_id)
DO UPDATE SET status = 'invited'
WHERE channel_members.status IN ('left', 'removed')
`, params.WorkspaceID, params.ChannelID, params.UserID)
	if err != nil {
		return channelsdomain.Member{}, err
	}
	if command.RowsAffected() == 0 {
		return channelsdomain.Member{}, channelsdomain.ErrMemberNotFound
	}
	return r.member(ctx, params.WorkspaceID, params.ChannelID, params.UserID)
}

func (r *Repository) UpdateMemberStatus(ctx context.Context, params channelsapp.UpdateMemberStatusParams) (channelsdomain.Member, error) {
	command, err := r.pool.Exec(ctx, `
UPDATE channel_members cm
SET status = $4
FROM channels c
WHERE c.id = cm.channel_id
  AND c.workspace_id = $1::uuid
  AND c.id = $2::uuid
  AND cm.user_id = $3::uuid
`, params.WorkspaceID, params.ChannelID, params.UserID, params.Status)
	if err != nil {
		return channelsdomain.Member{}, err
	}
	if command.RowsAffected() == 0 {
		return channelsdomain.Member{}, channelsdomain.ErrMemberNotFound
	}
	return r.member(ctx, params.WorkspaceID, params.ChannelID, params.UserID)
}

func (r *Repository) UpdateReadState(ctx context.Context, params channelsapp.UpdateReadStateParams) (channelsdomain.Member, error) {
	command, err := r.pool.Exec(ctx, `
UPDATE channel_members cm
SET last_read_at = now(),
    last_read_message_id = NULLIF($4, '')::uuid
FROM channels c
WHERE c.id = cm.channel_id
  AND c.workspace_id = $1::uuid
  AND c.id = $2::uuid
  AND cm.user_id = $3::uuid
`, params.WorkspaceID, params.ChannelID, params.UserID, params.LastReadMessageID)
	if err != nil {
		return channelsdomain.Member{}, err
	}
	if command.RowsAffected() == 0 {
		return channelsdomain.Member{}, channelsdomain.ErrMemberNotFound
	}
	return r.member(ctx, params.WorkspaceID, params.ChannelID, params.UserID)
}

func (r *Repository) CreateOrGetPrivateSession(ctx context.Context, params channelsapp.PrivateSessionParams) (channelsdomain.Channel, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return channelsdomain.Channel{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	lockKey := params.WorkspaceID + ":" + params.SourceChannelID + ":" + params.UserID
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, lockKey); err != nil {
		return channelsdomain.Channel{}, err
	}

	const selectSession = `
SELECT id::text, workspace_id::text, department_id::text, slug::text, name, description, type, status, created_by::text,
       created_at, updated_at, archived_at, COALESCE(settings->>'bot_session_mode', '') = 'private'
FROM channels
WHERE workspace_id = $1::uuid
  AND settings->>'bot_source_channel_id' = $2
  AND settings->>'bot_session_user_id' = $3
  AND settings @> '{"bot_session":true}'::jsonb
  AND deleted_at IS NULL
`
	existing, err := scanChannel(tx.QueryRow(ctx, selectSession, params.WorkspaceID, params.SourceChannelID, params.UserID))
	if err == nil {
		if err := ensureActiveChannelMember(ctx, tx, existing.ID, params.UserID); err != nil {
			return channelsdomain.Channel{}, err
		}
		if err := ensureBotChannelGuidePin(ctx, tx, params.WorkspaceID, existing.ID, params.UserID, params.SourceChannelID); err != nil {
			return channelsdomain.Channel{}, err
		}
		return existing, tx.Commit(ctx)
	}
	if !errors.Is(err, channelsdomain.ErrChannelNotFound) {
		return channelsdomain.Channel{}, err
	}

	row := tx.QueryRow(ctx, `
INSERT INTO channels (workspace_id, name, description, type, status, created_by, settings)
SELECT source.workspace_id,
       source.name,
       'Không gian làm việc riêng tư với ' || source.name,
       'direct',
       'active',
       $3::uuid,
       jsonb_build_object(
           'bot_session', true,
           'bot_source_channel_id', source.id::text,
           'bot_source_slug', source.slug::text,
           'bot_session_user_id', $3
       )
FROM channels source
JOIN workspace_members wm
  ON wm.workspace_id = source.workspace_id
 AND wm.user_id = $3::uuid
 AND wm.status = 'active'
WHERE source.workspace_id = $1::uuid
  AND source.id = $2::uuid
  AND source.settings->>'bot_session_mode' = 'private'
  AND source.status = 'active'
  AND source.deleted_at IS NULL
RETURNING id::text, workspace_id::text, department_id::text, slug::text, name, description, type, status, created_by::text,
          created_at, updated_at, archived_at, COALESCE(settings->>'bot_session_mode', '') = 'private'
`, params.WorkspaceID, params.SourceChannelID, params.UserID)
	session, err := scanChannel(row)
	if err != nil {
		return channelsdomain.Channel{}, err
	}

	if err := ensureActiveChannelMember(ctx, tx, session.ID, params.UserID); err != nil {
		return channelsdomain.Channel{}, err
	}
	if err := ensureBotChannelGuidePin(ctx, tx, params.WorkspaceID, session.ID, params.UserID, params.SourceChannelID); err != nil {
		return channelsdomain.Channel{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return channelsdomain.Channel{}, err
	}
	return session, nil
}

func ensureActiveChannelMember(ctx context.Context, exec commandExecutor, channelID, userID string) error {
	_, err := exec.Exec(ctx, `
INSERT INTO channel_members (channel_id, user_id, status)
VALUES ($1::uuid, $2::uuid, 'active')
ON CONFLICT (channel_id, user_id) DO UPDATE SET status = 'active'
`, channelID, userID)
	return err
}

func ensureBotChannelGuidePin(ctx context.Context, exec commandExecutor, workspaceID, channelID, userID, sourceChannelID string) error {
	_, err := exec.Exec(ctx, `
WITH source AS (
    SELECT slug::text AS slug
    FROM channels
    WHERE workspace_id = $1::uuid
      AND id = $4::uuid
      AND deleted_at IS NULL
),
guide AS (
    SELECT
        source.slug,
        CASE source.slug
            WHEN 'gia-han' THEN 'Kiểm tra dịch vụ sắp hết hạn
Email: khach@example.com
Số ngày: 30
Loại dịch vụ: VPS'
            WHEN 'ke-toan' THEN 'Tạo QR nạp ví
Email: khach@example.com
Số tiền: 200000'
            WHEN 'ticket' THEN 'Tạo ticket hỗ trợ cho khach@example.com
VPS #1234 bị mất kết nối, ping timeout và port 22 không truy cập được.'
            WHEN 'server-alert' THEN 'Server: vps-01
Lỗi: Mất ping 3 phút
IP: 192.0.2.10
Mức độ: critical'
        END AS body
    FROM source
    WHERE source.slug IN ('gia-han', 'ke-toan', 'ticket', 'server-alert')
),
existing AS (
    SELECT m.id, m.workspace_id, m.channel_id
    FROM messages m
    JOIN guide ON true
    WHERE m.workspace_id = $1::uuid
      AND m.channel_id = $2::uuid
      AND m.metadata @> jsonb_build_object(
          'seed', 'bot_channel_guide',
          'source_channel_id', $4::text
      )
      AND m.deleted_at IS NULL
    ORDER BY m.created_at ASC
    LIMIT 1
),
inserted AS (
    INSERT INTO messages (workspace_id, channel_id, sender_id, kind, body, metadata)
    SELECT
        $1::uuid,
        $2::uuid,
        $3::uuid,
        'text',
        guide.body,
        jsonb_build_object(
            'seed', 'bot_channel_guide',
            'source_channel_id', $4::text,
            'source_slug', guide.slug
        )
    FROM guide
    WHERE guide.body IS NOT NULL
      AND NOT EXISTS (SELECT 1 FROM existing)
    RETURNING id, workspace_id, channel_id
),
guide_message AS (
    SELECT id, workspace_id, channel_id FROM existing
    UNION ALL
    SELECT id, workspace_id, channel_id FROM inserted
)
INSERT INTO message_pins (workspace_id, channel_id, message_id, pinned_by)
SELECT workspace_id, channel_id, id, $3::uuid
FROM guide_message
ON CONFLICT (workspace_id, channel_id, message_id)
DO UPDATE SET pinned_by = EXCLUDED.pinned_by,
              created_at = message_pins.created_at
`, workspaceID, channelID, userID, sourceChannelID)
	return err
}

func (r *Repository) CreateOrGetDirectConversation(ctx context.Context, params channelsapp.CreateDirectParams) (channelsdomain.DirectConversation, error) {
	existing, err := r.directByKey(ctx, params.WorkspaceID, params.ParticipantKey)
	if err == nil {
		return existing, nil
	}
	if err != nil && !errors.Is(err, pgx.ErrNoRows) && !errors.Is(err, channelsdomain.ErrChannelNotFound) {
		return channelsdomain.DirectConversation{}, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return channelsdomain.DirectConversation{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	channelName := "Direct message"
	if params.ConversationType == "group" {
		channelName = "Group direct message"
	}
	var channelID string
	if err := tx.QueryRow(ctx, `
INSERT INTO channels (workspace_id, name, type, status, created_by)
VALUES ($1::uuid, $2, 'direct', 'active', $3::uuid)
RETURNING id::text
`, params.WorkspaceID, channelName, params.CreatedBy).Scan(&channelID); err != nil {
		return channelsdomain.DirectConversation{}, err
	}

	var directID string
	if err := tx.QueryRow(ctx, `
INSERT INTO direct_conversations (workspace_id, channel_id, participant_key, conversation_type, created_by)
VALUES ($1::uuid, $2::uuid, $3, $4, $5::uuid)
RETURNING id::text
`, params.WorkspaceID, channelID, params.ParticipantKey, params.ConversationType, params.CreatedBy).Scan(&directID); err != nil {
		if isUniqueViolation(err) {
			return r.directByKey(ctx, params.WorkspaceID, params.ParticipantKey)
		}
		return channelsdomain.DirectConversation{}, err
	}

	for _, userID := range params.ParticipantIDs {
		if _, err := tx.Exec(ctx, `
INSERT INTO workspace_members (workspace_id, user_id, status, joined_at)
SELECT $1::uuid, u.id, 'active', now()
FROM users u
WHERE u.id = $2::uuid AND u.deleted_at IS NULL AND u.status = 'active'
ON CONFLICT (workspace_id, user_id)
DO UPDATE SET status = 'active', joined_at = COALESCE(workspace_members.joined_at, now())
`, params.WorkspaceID, userID); err != nil {
			return channelsdomain.DirectConversation{}, err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO workspace_member_roles (workspace_id, user_id, role_id, assigned_by)
SELECT $1::uuid, $2::uuid, r.id, $3::uuid
FROM roles r
WHERE r.workspace_id IS NULL AND r.code = 'workspace_member' AND r.deleted_at IS NULL
ON CONFLICT (workspace_id, user_id, role_id)
DO UPDATE SET assigned_by = workspace_member_roles.assigned_by
`, params.WorkspaceID, userID, params.CreatedBy); err != nil {
			return channelsdomain.DirectConversation{}, err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO direct_conversation_members (direct_conversation_id, user_id)
SELECT $1::uuid, wm.user_id
FROM workspace_members wm
WHERE wm.workspace_id = $2::uuid AND wm.user_id = $3::uuid AND wm.status = 'active'
ON CONFLICT DO NOTHING
`, directID, params.WorkspaceID, userID); err != nil {
			return channelsdomain.DirectConversation{}, err
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO channel_members (channel_id, user_id, status)
SELECT $1::uuid, wm.user_id, 'active'
FROM workspace_members wm
WHERE wm.workspace_id = $2::uuid AND wm.user_id = $3::uuid AND wm.status = 'active'
ON CONFLICT (channel_id, user_id) DO UPDATE SET status = 'active'
`, channelID, params.WorkspaceID, userID); err != nil {
			return channelsdomain.DirectConversation{}, err
		}
	}

	var insertedCount int
	if err := tx.QueryRow(ctx, `
SELECT count(*)
FROM direct_conversation_members
WHERE direct_conversation_id = $1::uuid
`, directID).Scan(&insertedCount); err != nil {
		return channelsdomain.DirectConversation{}, err
	}
	if insertedCount != len(params.ParticipantIDs) {
		return channelsdomain.DirectConversation{}, channelsdomain.ErrMemberNotFound
	}

	if err := tx.Commit(ctx); err != nil {
		return channelsdomain.DirectConversation{}, err
	}
	return r.directByID(ctx, directID)
}

func (r *Repository) HasAcceptedContact(ctx context.Context, actorUserID string, participantUserID string) (bool, error) {
	var accepted bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM contact_requests
    WHERE deleted_at IS NULL
      AND status = 'accepted'
      AND LEAST(requester_id, receiver_id) = LEAST($1::uuid, $2::uuid)
      AND GREATEST(requester_id, receiver_id) = GREATEST($1::uuid, $2::uuid)
)
`, actorUserID, participantUserID).Scan(&accepted)
	return accepted, err
}

func (r *Repository) ListDirectConversations(ctx context.Context, workspaceID string, userID string) ([]channelsdomain.DirectConversation, error) {
	rows, err := r.pool.Query(ctx, `
SELECT dc.id::text, dc.workspace_id::text, dc.channel_id::text, dc.participant_key, dc.conversation_type,
       dc.created_by::text, dc.created_at, dc.updated_at,
       lm.id::text, lm.workspace_id::text, lm.channel_id::text, lm.sender_id::text,
       lm.kind, lm.body, lm.created_at, lm.updated_at,
       COALESCE(unread.unread_count, 0)
FROM direct_conversations dc
JOIN direct_conversation_members dcm ON dcm.direct_conversation_id = dc.id AND dcm.user_id = $2::uuid
LEFT JOIN channel_members actor_member ON actor_member.channel_id = dc.channel_id AND actor_member.user_id = $2::uuid
LEFT JOIN LATERAL (
    SELECT m.id, m.workspace_id, m.channel_id, m.sender_id, m.kind, m.body, m.created_at, m.updated_at
    FROM messages m
    WHERE m.workspace_id = dc.workspace_id
      AND m.channel_id = dc.channel_id
      AND m.deleted_at IS NULL
    ORDER BY m.created_at DESC, m.id DESC
    LIMIT 1
) lm ON true
LEFT JOIN messages read_marker
  ON read_marker.workspace_id = dc.workspace_id
 AND read_marker.id = actor_member.last_read_message_id
LEFT JOIN LATERAL (
    SELECT count(*)::int AS unread_count
    FROM messages m
    WHERE m.workspace_id = dc.workspace_id
      AND m.channel_id = dc.channel_id
      AND m.deleted_at IS NULL
      AND (m.sender_id IS NULL OR m.sender_id <> $2::uuid)
      AND m.created_at > COALESCE(read_marker.created_at, actor_member.last_read_at, actor_member.joined_at, '-infinity'::timestamptz)
) unread ON true
WHERE dc.workspace_id = $1::uuid AND dc.archived_at IS NULL
ORDER BY COALESCE(lm.created_at, dc.updated_at) DESC, dc.updated_at DESC
`, workspaceID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []channelsdomain.DirectConversation
	for rows.Next() {
		conversation, err := scanDirectWithSummary(rows)
		if err != nil {
			return nil, err
		}
		conversation.Participants, err = r.directParticipants(ctx, conversation.ID)
		if err != nil {
			return nil, err
		}
		conversation.ParticipantIDs = memberIDs(conversation.Participants)
		conversations = append(conversations, conversation)
	}
	return conversations, rows.Err()
}

func (r *Repository) RecordAudit(ctx context.Context, event channelsapp.AuditEvent) error {
	metadata := event.Metadata
	if metadata == nil {
		metadata = map[string]any{}
	}
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, `
INSERT INTO audit_logs (workspace_id, actor_user_id, action, entity_type, entity_id, metadata)
VALUES (NULLIF($1, '')::uuid, NULLIF($2, '')::uuid, $3, $4, NULLIF($5, '')::uuid, $6::jsonb)
`, event.WorkspaceID, event.ActorUserID, event.Action, event.EntityType, event.EntityID, string(metadataBytes))
	return err
}

func (r *Repository) member(ctx context.Context, workspaceID string, channelID string, userID string) (channelsdomain.Member, error) {
	row := r.pool.QueryRow(ctx, `
SELECT cm.channel_id::text, cm.user_id::text, u.email::text, u.username::text, u.display_name, u.avatar_url,
       cm.status, cm.last_read_at, cm.last_read_message_id::text, cm.joined_at, cm.created_at, cm.updated_at
FROM channel_members cm
JOIN channels c ON c.id = cm.channel_id AND c.deleted_at IS NULL
JOIN users u ON u.id = cm.user_id AND u.deleted_at IS NULL
WHERE c.workspace_id = $1::uuid AND c.id = $2::uuid AND cm.user_id = $3::uuid
`, workspaceID, channelID, userID)
	return scanMember(row)
}

func (r *Repository) directByKey(ctx context.Context, workspaceID string, participantKey string) (channelsdomain.DirectConversation, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id::text, workspace_id::text, channel_id::text, participant_key, conversation_type,
       created_by::text, created_at, updated_at
FROM direct_conversations
WHERE workspace_id = $1::uuid AND participant_key = $2 AND archived_at IS NULL
`, workspaceID, participantKey)
	conversation, err := scanDirect(row)
	if err != nil {
		return channelsdomain.DirectConversation{}, err
	}
	conversation.Participants, err = r.directParticipants(ctx, conversation.ID)
	conversation.ParticipantIDs = memberIDs(conversation.Participants)
	return conversation, err
}

func (r *Repository) directByID(ctx context.Context, directID string) (channelsdomain.DirectConversation, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id::text, workspace_id::text, channel_id::text, participant_key, conversation_type,
       created_by::text, created_at, updated_at
FROM direct_conversations
WHERE id = $1::uuid
`, directID)
	conversation, err := scanDirect(row)
	if err != nil {
		return channelsdomain.DirectConversation{}, err
	}
	conversation.Participants, err = r.directParticipants(ctx, conversation.ID)
	conversation.ParticipantIDs = memberIDs(conversation.Participants)
	return conversation, err
}

func (r *Repository) directParticipants(ctx context.Context, directID string) ([]channelsdomain.Member, error) {
	rows, err := r.pool.Query(ctx, `
SELECT dc.channel_id::text, dcm.user_id::text, u.email::text, u.username::text, u.display_name, u.avatar_url,
       cm.status, cm.last_read_at, cm.last_read_message_id::text, cm.joined_at, cm.created_at, cm.updated_at
FROM direct_conversation_members dcm
JOIN direct_conversations dc ON dc.id = dcm.direct_conversation_id
JOIN users u ON u.id = dcm.user_id AND u.deleted_at IS NULL
LEFT JOIN channel_members cm ON cm.channel_id = dc.channel_id AND cm.user_id = dcm.user_id
WHERE dcm.direct_conversation_id = $1::uuid
ORDER BY u.display_name
`, directID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []channelsdomain.Member
	for rows.Next() {
		member, err := scanMember(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

func memberIDs(members []channelsdomain.Member) []string {
	ids := make([]string, 0, len(members))
	for _, member := range members {
		ids = append(ids, member.UserID)
	}
	return ids
}

type rowScanner interface {
	Scan(dest ...any) error
}

type commandExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func scanChannel(row rowScanner) (channelsdomain.Channel, error) {
	var channel channelsdomain.Channel
	var departmentID sql.NullString
	var slug sql.NullString
	var description sql.NullString
	var createdBy sql.NullString
	var archivedAt sql.NullTime
	if err := row.Scan(
		&channel.ID,
		&channel.WorkspaceID,
		&departmentID,
		&slug,
		&channel.Name,
		&description,
		&channel.Type,
		&channel.Status,
		&createdBy,
		&channel.CreatedAt,
		&channel.UpdatedAt,
		&archivedAt,
		&channel.PrivateSessionMode,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return channelsdomain.Channel{}, channelsdomain.ErrChannelNotFound
		}
		return channelsdomain.Channel{}, err
	}
	channel.DepartmentID = nullStringPtr(departmentID)
	channel.Slug = nullStringPtr(slug)
	channel.Description = nullStringPtr(description)
	channel.CreatedBy = nullStringPtr(createdBy)
	channel.ArchivedAt = nullTimePtr(archivedAt)
	return channel, nil
}

func scanMember(row rowScanner) (channelsdomain.Member, error) {
	var member channelsdomain.Member
	var avatarURL sql.NullString
	var lastReadAt sql.NullTime
	var lastReadMessageID sql.NullString
	if err := row.Scan(
		&member.ChannelID,
		&member.UserID,
		&member.Email,
		&member.Username,
		&member.DisplayName,
		&avatarURL,
		&member.Status,
		&lastReadAt,
		&lastReadMessageID,
		&member.JoinedAt,
		&member.CreatedAt,
		&member.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return channelsdomain.Member{}, channelsdomain.ErrMemberNotFound
		}
		return channelsdomain.Member{}, err
	}
	member.AvatarURL = nullStringPtr(avatarURL)
	member.LastReadAt = nullTimePtr(lastReadAt)
	member.LastReadMessageID = nullStringPtr(lastReadMessageID)
	return member, nil
}

func scanDirect(row rowScanner) (channelsdomain.DirectConversation, error) {
	var conversation channelsdomain.DirectConversation
	var createdBy sql.NullString
	if err := row.Scan(
		&conversation.ID,
		&conversation.WorkspaceID,
		&conversation.ChannelID,
		&conversation.ParticipantKey,
		&conversation.ConversationType,
		&createdBy,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return channelsdomain.DirectConversation{}, fmt.Errorf("%w", channelsdomain.ErrChannelNotFound)
		}
		return channelsdomain.DirectConversation{}, err
	}
	conversation.CreatedBy = nullStringPtr(createdBy)
	return conversation, nil
}

func scanDirectWithSummary(row rowScanner) (channelsdomain.DirectConversation, error) {
	var conversation channelsdomain.DirectConversation
	var createdBy sql.NullString
	var messageID sql.NullString
	var messageWorkspaceID sql.NullString
	var messageChannelID sql.NullString
	var messageSenderID sql.NullString
	var messageKind sql.NullString
	var messageBody sql.NullString
	var messageCreatedAt sql.NullTime
	var messageUpdatedAt sql.NullTime
	if err := row.Scan(
		&conversation.ID,
		&conversation.WorkspaceID,
		&conversation.ChannelID,
		&conversation.ParticipantKey,
		&conversation.ConversationType,
		&createdBy,
		&conversation.CreatedAt,
		&conversation.UpdatedAt,
		&messageID,
		&messageWorkspaceID,
		&messageChannelID,
		&messageSenderID,
		&messageKind,
		&messageBody,
		&messageCreatedAt,
		&messageUpdatedAt,
		&conversation.UnreadCount,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return channelsdomain.DirectConversation{}, fmt.Errorf("%w", channelsdomain.ErrChannelNotFound)
		}
		return channelsdomain.DirectConversation{}, err
	}
	conversation.CreatedBy = nullStringPtr(createdBy)
	if messageID.Valid && messageCreatedAt.Valid && messageUpdatedAt.Valid {
		conversation.LastMessage = &channelsdomain.MessageSummary{
			ID:          messageID.String,
			WorkspaceID: messageWorkspaceID.String,
			ChannelID:   messageChannelID.String,
			SenderID:    nullStringPtr(messageSenderID),
			Kind:        messageKind.String,
			Body:        messageBody.String,
			CreatedAt:   messageCreatedAt.Time,
			UpdatedAt:   messageUpdatedAt.Time,
		}
	}
	return conversation, nil
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
