package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	messagesapp "github.com/duclamdev/application-chat/backend/internal/modules/messages/application"
	messagesdomain "github.com/duclamdev/application-chat/backend/internal/modules/messages/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

type commandExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Send(ctx context.Context, params messagesapp.SendParams) (messagesdomain.Message, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return messagesdomain.Message{}, fmt.Errorf("message send begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	threadRootID := ""
	if params.ParentID != "" {
		if err := tx.QueryRow(ctx, `
SELECT COALESCE(thread_root_id, id)::text
FROM messages
WHERE workspace_id = $1::uuid
  AND channel_id = $2::uuid
  AND id = $3::uuid
  AND deleted_at IS NULL
`, params.WorkspaceID, params.ChannelID, params.ParentID).Scan(&threadRootID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return messagesdomain.Message{}, messagesdomain.ErrMessageNotFound
			}
			return messagesdomain.Message{}, err
		}
	}

	if err := ensureDirectChannelMember(ctx, tx, params.WorkspaceID, params.ChannelID, params.SenderID); err != nil {
		return messagesdomain.Message{}, fmt.Errorf("message send repair direct channel members: %w", err)
	}

	if params.ClientMessageID != "" {
		existing, err := r.findByClientMessageID(ctx, tx, params)
		if err == nil {
			return existing, nil
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return messagesdomain.Message{}, fmt.Errorf("message send check idempotency: %w", err)
		}
	}

	row := tx.QueryRow(ctx, `
INSERT INTO messages (workspace_id, channel_id, sender_id, parent_id, thread_root_id, kind, body, metadata)
SELECT c.workspace_id, c.id, $3::uuid, NULLIF($4, '')::uuid, NULLIF($5, '')::uuid, $6, $7, $8::jsonb
FROM channels c
JOIN channel_members cm
  ON cm.channel_id = c.id
 AND cm.user_id = $3::uuid
 AND cm.status IN ('active', 'muted')
WHERE c.workspace_id = $1::uuid
  AND c.id = $2::uuid
  AND c.deleted_at IS NULL
  AND c.status = 'active'
RETURNING id::text, workspace_id::text, channel_id::text, sender_id::text, parent_id::text,
          thread_root_id::text, kind, body, metadata::text, edited_at, deleted_at, created_at, updated_at
`, params.WorkspaceID, params.ChannelID, params.SenderID, params.ParentID, threadRootID, params.Kind, params.Body, string(params.Metadata))
	message, err := scanMessage(row)
	if err != nil {
		if params.ClientMessageID != "" && isUniqueViolation(err) {
			_ = tx.Rollback(ctx)
			return r.findByClientMessageID(ctx, r.pool, params)
		}
		if errors.Is(err, messagesdomain.ErrMessageNotFound) {
			return messagesdomain.Message{}, messagesdomain.ErrChannelNotFound
		}
		return messagesdomain.Message{}, fmt.Errorf("message send insert message: %w", err)
	}

	if err := touchDirectConversationLastMessage(ctx, tx, message); err != nil {
		return messagesdomain.Message{}, fmt.Errorf("message send touch direct conversation: %w", err)
	}
	if err := r.replaceMentions(ctx, tx, message.WorkspaceID, message.ChannelID, message.ID, params.MentionedUserIDs); err != nil {
		return messagesdomain.Message{}, fmt.Errorf("message send replace mentions: %w", err)
	}
	if err := upsertSearchDocument(ctx, tx, message); err != nil {
		return messagesdomain.Message{}, fmt.Errorf("message send upsert search document: %w", err)
	}
	if err := insertOutbox(ctx, tx, "message", message.ID, "MessageCreated", map[string]any{
		"workspace_id":       message.WorkspaceID,
		"channel_id":         message.ChannelID,
		"message_id":         message.ID,
		"sender_id":          params.SenderID,
		"parent_id":          params.ParentID,
		"thread_root_id":     threadRootID,
		"mentioned_user_ids": params.MentionedUserIDs,
	}); err != nil {
		return messagesdomain.Message{}, fmt.Errorf("message send insert outbox: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return messagesdomain.Message{}, fmt.Errorf("message send commit: %w", err)
	}
	hydrated, err := r.Get(ctx, messagesapp.MessageRef{
		WorkspaceID: params.WorkspaceID,
		ChannelID:   params.ChannelID,
		MessageID:   message.ID,
		ActorUserID: params.SenderID,
	})
	if err != nil {
		slog.Warn("Khong hydrate duoc tin nhan vua luu, tra ve payload toi thieu",
			"workspace_id", params.WorkspaceID,
			"channel_id", params.ChannelID,
			"message_id", message.ID,
			"actor_user_id", params.SenderID,
			"error", err,
		)
		return withEmptyMessageRelations(message), nil
	}
	return hydrated, nil
}

func (r *Repository) findByClientMessageID(ctx context.Context, querier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, params messagesapp.SendParams) (messagesdomain.Message, error) {
	var messageID string
	if err := querier.QueryRow(ctx, `
SELECT id::text
FROM messages
WHERE workspace_id = $1::uuid
  AND channel_id = $2::uuid
  AND sender_id = $3::uuid
  AND metadata ->> 'client_message_id' = $4
  AND deleted_at IS NULL
ORDER BY created_at DESC
LIMIT 1
`, params.WorkspaceID, params.ChannelID, params.SenderID, params.ClientMessageID).Scan(&messageID); err != nil {
		return messagesdomain.Message{}, err
	}
	return r.Get(ctx, messagesapp.MessageRef{
		WorkspaceID: params.WorkspaceID,
		ChannelID:   params.ChannelID,
		MessageID:   messageID,
		ActorUserID: params.SenderID,
	})
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func withEmptyMessageRelations(message messagesdomain.Message) messagesdomain.Message {
	if message.Mentions == nil {
		message.Mentions = []string{}
	}
	if message.Reactions == nil {
		message.Reactions = []messagesdomain.ReactionSummary{}
	}
	return message
}

func ensureDirectChannelMember(ctx context.Context, exec commandExecutor, workspaceID, channelID, userID string) error {
	_, err := exec.Exec(ctx, `
INSERT INTO channel_members (channel_id, user_id, status)
SELECT DISTINCT dc.channel_id, dcm.user_id, 'active'
FROM direct_conversations dc
JOIN direct_conversation_members dcm ON dcm.direct_conversation_id = dc.id
WHERE dc.workspace_id = $1::uuid
  AND dc.channel_id = $2::uuid
  AND EXISTS (
      SELECT 1
      FROM direct_conversation_members sender_member
      WHERE sender_member.direct_conversation_id = dc.id
        AND sender_member.user_id = $3::uuid
  )
ORDER BY dcm.user_id
ON CONFLICT (channel_id, user_id)
DO UPDATE SET status = 'active'
WHERE channel_members.status IN ('left', 'removed', 'invited')
`, workspaceID, channelID, userID)
	return err
}

func touchDirectConversationLastMessage(ctx context.Context, exec commandExecutor, message messagesdomain.Message) error {
	_, err := exec.Exec(ctx, `
UPDATE direct_conversations
SET last_message_id = $3::uuid,
    updated_at = GREATEST(updated_at, $4::timestamptz)
WHERE workspace_id = $1::uuid
  AND channel_id = $2::uuid
  AND archived_at IS NULL
`, message.WorkspaceID, message.ChannelID, message.ID, message.CreatedAt)
	return err
}

func (r *Repository) Get(ctx context.Context, params messagesapp.MessageRef) (messagesdomain.Message, error) {
	row := r.pool.QueryRow(ctx, `
SELECT m.id::text, m.workspace_id::text, m.channel_id::text, m.sender_id::text, m.parent_id::text,
       m.thread_root_id::text, m.kind, m.body, m.metadata::text, m.edited_at, m.deleted_at, m.created_at, m.updated_at
FROM messages m
JOIN channel_members cm
  ON cm.channel_id = m.channel_id
 AND cm.user_id = $4::uuid
 AND cm.status IN ('active', 'muted')
WHERE m.workspace_id = $1::uuid
  AND m.channel_id = $2::uuid
  AND m.id = $3::uuid
  AND m.deleted_at IS NULL
`, params.WorkspaceID, params.ChannelID, params.MessageID, params.ActorUserID)
	message, err := scanMessage(row)
	if err != nil {
		return messagesdomain.Message{}, err
	}
	return r.hydrateMessage(ctx, message, params.ActorUserID)
}

func (r *Repository) List(ctx context.Context, params messagesapp.ListParams) ([]messagesdomain.Message, error) {
	rows, err := r.pool.Query(ctx, `
WITH boundary AS (
    SELECT created_at, id
    FROM messages
    WHERE workspace_id = $1::uuid AND id = NULLIF($5, '')::uuid
)
SELECT m.id::text, m.workspace_id::text, m.channel_id::text, m.sender_id::text, m.parent_id::text,
       m.thread_root_id::text, m.kind, m.body, m.metadata::text, m.edited_at, m.deleted_at, m.created_at, m.updated_at
FROM messages m
JOIN channel_members cm
  ON cm.channel_id = m.channel_id
 AND cm.user_id = $3::uuid
 AND cm.status IN ('active', 'muted')
WHERE m.workspace_id = $1::uuid
  AND m.channel_id = $2::uuid
  AND m.deleted_at IS NULL
  AND (
      $5 = ''
      OR EXISTS (
          SELECT 1
          FROM boundary b
          WHERE (m.created_at, m.id) < (b.created_at, b.id)
      )
  )
ORDER BY m.created_at DESC, m.id DESC
LIMIT $4
`, params.WorkspaceID, params.ChannelID, params.ActorUserID, params.Limit, params.BeforeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}
	return r.hydrateMessages(ctx, messages, params.ActorUserID)
}

func (r *Repository) ListThread(ctx context.Context, params messagesapp.ThreadParams) ([]messagesdomain.Message, error) {
	rows, err := r.pool.Query(ctx, `
WITH root AS (
    SELECT COALESCE(thread_root_id, id) AS id
    FROM messages
    WHERE workspace_id = $1::uuid
      AND channel_id = $2::uuid
      AND id = $4::uuid
      AND deleted_at IS NULL
)
SELECT m.id::text, m.workspace_id::text, m.channel_id::text, m.sender_id::text, m.parent_id::text,
       m.thread_root_id::text, m.kind, m.body, m.metadata::text, m.edited_at, m.deleted_at, m.created_at, m.updated_at
FROM messages m
JOIN channel_members cm
  ON cm.channel_id = m.channel_id
 AND cm.user_id = $3::uuid
 AND cm.status IN ('active', 'muted')
WHERE m.workspace_id = $1::uuid
  AND m.channel_id = $2::uuid
  AND m.deleted_at IS NULL
  AND EXISTS (
      SELECT 1
      FROM root r
      WHERE m.id = r.id OR m.thread_root_id = r.id
  )
ORDER BY m.created_at ASC, m.id ASC
LIMIT $5
`, params.WorkspaceID, params.ChannelID, params.ActorUserID, params.MessageID, params.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, messagesdomain.ErrMessageNotFound
	}
	return r.hydrateMessages(ctx, messages, params.ActorUserID)
}

func (r *Repository) Search(ctx context.Context, params messagesapp.SearchParams) ([]messagesdomain.Message, error) {
	rows, err := r.pool.Query(ctx, `
SELECT m.id::text, m.workspace_id::text, m.channel_id::text, m.sender_id::text, m.parent_id::text,
       m.thread_root_id::text, m.kind, m.body, m.metadata::text, m.edited_at, m.deleted_at, m.created_at, m.updated_at
FROM messages m
JOIN channel_members cm
  ON cm.channel_id = m.channel_id
 AND cm.user_id = $2::uuid
 AND cm.status IN ('active', 'muted')
WHERE m.workspace_id = $1::uuid
  AND m.deleted_at IS NULL
  AND m.search_vector @@ plainto_tsquery('simple', $3)
	AND ($5 = '' OR m.channel_id = NULLIF($5, '')::uuid)
	AND ($6 = '' OR m.sender_id = NULLIF($6, '')::uuid)
	AND ($7 = '' OR m.kind = $7)
	AND ($8::timestamptz IS NULL OR m.created_at >= $8::timestamptz)
	AND ($9::timestamptz IS NULL OR m.created_at < $9::timestamptz)
ORDER BY ts_rank(m.search_vector, plainto_tsquery('simple', $3)) DESC, m.created_at DESC
LIMIT $4
`, params.WorkspaceID, params.ActorUserID, params.Query, params.Limit, params.ChannelID, params.SenderID, params.Kind, params.DateFrom, params.DateTo)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}
	return r.hydrateMessages(ctx, messages, params.ActorUserID)
}

func (r *Repository) Forward(ctx context.Context, params messagesapp.ForwardParams) (messagesdomain.Message, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return messagesdomain.Message{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
WITH source_message AS (
    SELECT m.*
    FROM messages m
    JOIN channel_members source_member
      ON source_member.channel_id = m.channel_id
     AND source_member.user_id = $5::uuid
     AND source_member.status IN ('active', 'muted')
    WHERE m.workspace_id = $1::uuid
      AND m.channel_id = $2::uuid
      AND m.id = $3::uuid
      AND m.deleted_at IS NULL
), target_channel AS (
    SELECT c.id, c.workspace_id
    FROM channels c
    JOIN channel_members target_member
      ON target_member.channel_id = c.id
     AND target_member.user_id = $5::uuid
     AND target_member.status IN ('active', 'muted')
    WHERE c.workspace_id = $1::uuid
      AND c.id = $4::uuid
      AND c.status = 'active'
      AND c.deleted_at IS NULL
)
INSERT INTO messages (workspace_id, channel_id, sender_id, kind, body, metadata)
SELECT target.workspace_id,
       target.id,
       $5::uuid,
       source.kind,
       source.body,
       source.metadata || jsonb_build_object(
           'forwarded_from', jsonb_build_object(
               'message_id', source.id,
               'channel_id', source.channel_id,
               'sender_id', source.sender_id
           )
       )
FROM source_message source
CROSS JOIN target_channel target
RETURNING id::text, workspace_id::text, channel_id::text, sender_id::text, parent_id::text,
          thread_root_id::text, kind, body, metadata::text, edited_at, deleted_at, created_at, updated_at
`, params.WorkspaceID, params.SourceChannelID, params.MessageID, params.TargetChannelID, params.ActorUserID)
	message, err := scanMessage(row)
	if err != nil {
		return messagesdomain.Message{}, err
	}

	if err := touchDirectConversationLastMessage(ctx, tx, message); err != nil {
		return messagesdomain.Message{}, fmt.Errorf("message forward touch direct conversation: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO message_attachments (workspace_id, message_id, file_id, sort_order)
SELECT workspace_id, $3::uuid, file_id, sort_order
FROM message_attachments
WHERE workspace_id = $1::uuid AND message_id = $2::uuid
`, params.WorkspaceID, params.MessageID, message.ID); err != nil {
		return messagesdomain.Message{}, err
	}
	if err := upsertSearchDocument(ctx, tx, message); err != nil {
		return messagesdomain.Message{}, err
	}
	if err := insertOutbox(ctx, tx, "message", message.ID, "MessageCreated", map[string]any{
		"workspace_id":         message.WorkspaceID,
		"channel_id":           message.ChannelID,
		"message_id":           message.ID,
		"sender_id":            params.ActorUserID,
		"forwarded_message_id": params.MessageID,
	}); err != nil {
		return messagesdomain.Message{}, err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (workspace_id, actor_user_id, action, entity_type, entity_id, metadata)
VALUES ($1::uuid, $2::uuid, 'message.forward', 'message', $3::uuid,
        jsonb_build_object('source_message_id', $4, 'source_channel_id', $5, 'target_channel_id', $6))
`, params.WorkspaceID, params.ActorUserID, message.ID, params.MessageID, params.SourceChannelID, params.TargetChannelID); err != nil {
		return messagesdomain.Message{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return messagesdomain.Message{}, err
	}
	return r.Get(ctx, messagesapp.MessageRef{
		WorkspaceID: params.WorkspaceID,
		ChannelID:   params.TargetChannelID,
		MessageID:   message.ID,
		ActorUserID: params.ActorUserID,
	})
}

func (r *Repository) Update(ctx context.Context, params messagesapp.UpdateParams) (messagesdomain.Message, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return messagesdomain.Message{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	row := tx.QueryRow(ctx, `
UPDATE messages m
SET body = $5,
    edited_at = now()
FROM channel_members cm
WHERE cm.channel_id = m.channel_id
  AND cm.user_id = $4::uuid
  AND cm.status IN ('active', 'muted')
  AND m.workspace_id = $1::uuid
  AND m.channel_id = $2::uuid
  AND m.id = $3::uuid
  AND m.deleted_at IS NULL
RETURNING m.id::text, m.workspace_id::text, m.channel_id::text, m.sender_id::text, m.parent_id::text,
          m.thread_root_id::text, m.kind, m.body, m.metadata::text, m.edited_at, m.deleted_at, m.created_at, m.updated_at
`, params.WorkspaceID, params.ChannelID, params.MessageID, params.ActorUserID, params.Body)
	message, err := scanMessage(row)
	if err != nil {
		return messagesdomain.Message{}, err
	}

	if err := r.replaceMentions(ctx, tx, message.WorkspaceID, message.ChannelID, message.ID, params.MentionedUserIDs); err != nil {
		return messagesdomain.Message{}, err
	}
	if err := upsertSearchDocument(ctx, tx, message); err != nil {
		return messagesdomain.Message{}, err
	}
	if err := insertOutbox(ctx, tx, "message", message.ID, "MessageUpdated", map[string]any{
		"workspace_id":  message.WorkspaceID,
		"channel_id":    message.ChannelID,
		"message_id":    message.ID,
		"actor_user_id": params.ActorUserID,
	}); err != nil {
		return messagesdomain.Message{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return messagesdomain.Message{}, err
	}
	return r.Get(ctx, messagesapp.MessageRef{
		WorkspaceID: params.WorkspaceID,
		ChannelID:   params.ChannelID,
		MessageID:   params.MessageID,
		ActorUserID: params.ActorUserID,
	})
}

func (r *Repository) Delete(ctx context.Context, params messagesapp.DeleteParams) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	command, err := tx.Exec(ctx, `
UPDATE messages m
SET deleted_at = now()
WHERE m.workspace_id = $1::uuid
  AND m.channel_id = $2::uuid
  AND m.id = $3::uuid
  AND m.deleted_at IS NULL
`, params.WorkspaceID, params.ChannelID, params.MessageID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return messagesdomain.ErrMessageNotFound
	}

	if _, err := tx.Exec(ctx, `
DELETE FROM search_documents
WHERE workspace_id = $1::uuid AND source_type = 'message' AND source_id = $2::uuid
`, params.WorkspaceID, params.MessageID); err != nil {
		return err
	}
	if err := insertOutbox(ctx, tx, "message", params.MessageID, "MessageDeleted", map[string]any{
		"workspace_id":  params.WorkspaceID,
		"channel_id":    params.ChannelID,
		"message_id":    params.MessageID,
		"actor_user_id": params.ActorUserID,
	}); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (workspace_id, actor_user_id, action, entity_type, entity_id, metadata)
VALUES ($1::uuid, $4::uuid, 'message.delete', 'message', $3::uuid, jsonb_build_object('channel_id', $2))
`, params.WorkspaceID, params.ChannelID, params.MessageID, params.ActorUserID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *Repository) ListPins(ctx context.Context, params messagesapp.ListPinsParams) ([]messagesdomain.Message, error) {
	rows, err := r.pool.Query(ctx, `
SELECT m.id::text, m.workspace_id::text, m.channel_id::text, m.sender_id::text, m.parent_id::text,
       m.thread_root_id::text, m.kind, m.body, m.metadata::text, m.edited_at, m.deleted_at, m.created_at, m.updated_at
FROM message_pins mp
JOIN messages m
  ON m.workspace_id = mp.workspace_id
 AND m.id = mp.message_id
 AND m.deleted_at IS NULL
JOIN channel_members cm
  ON cm.channel_id = mp.channel_id
 AND cm.user_id = $3::uuid
 AND cm.status IN ('active', 'muted')
WHERE mp.workspace_id = $1::uuid
  AND mp.channel_id = $2::uuid
ORDER BY mp.created_at DESC
`, params.WorkspaceID, params.ChannelID, params.ActorUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}
	return r.hydrateMessages(ctx, messages, params.ActorUserID)
}

func (r *Repository) Pin(ctx context.Context, params messagesapp.PinParams) (messagesdomain.Message, error) {
	if _, err := r.Get(ctx, messagesapp.MessageRef{
		WorkspaceID: params.WorkspaceID,
		ChannelID:   params.ChannelID,
		MessageID:   params.MessageID,
		ActorUserID: params.ActorUserID,
	}); err != nil {
		return messagesdomain.Message{}, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return messagesdomain.Message{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if _, err := tx.Exec(ctx, `
INSERT INTO message_pins (workspace_id, channel_id, message_id, pinned_by)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4::uuid)
ON CONFLICT (workspace_id, channel_id, message_id)
DO UPDATE SET pinned_by = EXCLUDED.pinned_by,
              created_at = message_pins.created_at
`, params.WorkspaceID, params.ChannelID, params.MessageID, params.ActorUserID); err != nil {
		return messagesdomain.Message{}, err
	}
	if err := insertOutbox(ctx, tx, "message", params.MessageID, "MessagePinned", map[string]any{
		"workspace_id":  params.WorkspaceID,
		"channel_id":    params.ChannelID,
		"message_id":    params.MessageID,
		"actor_user_id": params.ActorUserID,
	}); err != nil {
		return messagesdomain.Message{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return messagesdomain.Message{}, err
	}
	return r.Get(ctx, messagesapp.MessageRef{
		WorkspaceID: params.WorkspaceID,
		ChannelID:   params.ChannelID,
		MessageID:   params.MessageID,
		ActorUserID: params.ActorUserID,
	})
}

func (r *Repository) Unpin(ctx context.Context, params messagesapp.PinParams) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	command, err := tx.Exec(ctx, `
DELETE FROM message_pins mp
USING channel_members cm
WHERE mp.workspace_id = $1::uuid
  AND mp.channel_id = $2::uuid
  AND mp.message_id = $3::uuid
  AND cm.channel_id = mp.channel_id
  AND cm.user_id = $4::uuid
  AND cm.status IN ('active', 'muted')
`, params.WorkspaceID, params.ChannelID, params.MessageID, params.ActorUserID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return messagesdomain.ErrPinNotFound
	}
	if err := insertOutbox(ctx, tx, "message", params.MessageID, "MessageUnpinned", map[string]any{
		"workspace_id":  params.WorkspaceID,
		"channel_id":    params.ChannelID,
		"message_id":    params.MessageID,
		"actor_user_id": params.ActorUserID,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) AddReaction(ctx context.Context, params messagesapp.ReactionParams) (messagesdomain.Message, error) {
	if _, err := r.Get(ctx, messagesapp.MessageRef{
		WorkspaceID: params.WorkspaceID,
		ChannelID:   params.ChannelID,
		MessageID:   params.MessageID,
		ActorUserID: params.ActorUserID,
	}); err != nil {
		return messagesdomain.Message{}, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return messagesdomain.Message{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	command, err := tx.Exec(ctx, `
INSERT INTO message_reactions (workspace_id, message_id, user_id, emoji)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4)
ON CONFLICT DO NOTHING
`, params.WorkspaceID, params.MessageID, params.ActorUserID, params.Emoji)
	if err != nil {
		return messagesdomain.Message{}, err
	}
	if command.RowsAffected() > 0 {
		if err := insertOutbox(ctx, tx, "message", params.MessageID, "ReactionChanged", map[string]any{
			"workspace_id":  params.WorkspaceID,
			"channel_id":    params.ChannelID,
			"message_id":    params.MessageID,
			"actor_user_id": params.ActorUserID,
			"emoji":         params.Emoji,
			"action":        "add",
		}); err != nil {
			return messagesdomain.Message{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return messagesdomain.Message{}, err
	}
	return r.Get(ctx, messagesapp.MessageRef{
		WorkspaceID: params.WorkspaceID,
		ChannelID:   params.ChannelID,
		MessageID:   params.MessageID,
		ActorUserID: params.ActorUserID,
	})
}

func (r *Repository) RemoveReaction(ctx context.Context, params messagesapp.ReactionParams) (messagesdomain.Message, error) {
	if _, err := r.Get(ctx, messagesapp.MessageRef{
		WorkspaceID: params.WorkspaceID,
		ChannelID:   params.ChannelID,
		MessageID:   params.MessageID,
		ActorUserID: params.ActorUserID,
	}); err != nil {
		return messagesdomain.Message{}, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return messagesdomain.Message{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	command, err := tx.Exec(ctx, `
DELETE FROM message_reactions
WHERE workspace_id = $1::uuid
  AND message_id = $2::uuid
  AND user_id = $3::uuid
  AND emoji = $4
`, params.WorkspaceID, params.MessageID, params.ActorUserID, params.Emoji)
	if err != nil {
		return messagesdomain.Message{}, err
	}
	if command.RowsAffected() == 0 {
		return messagesdomain.Message{}, messagesdomain.ErrReactionNotFound
	}
	if err := insertOutbox(ctx, tx, "message", params.MessageID, "ReactionChanged", map[string]any{
		"workspace_id":  params.WorkspaceID,
		"channel_id":    params.ChannelID,
		"message_id":    params.MessageID,
		"actor_user_id": params.ActorUserID,
		"emoji":         params.Emoji,
		"action":        "remove",
	}); err != nil {
		return messagesdomain.Message{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return messagesdomain.Message{}, err
	}
	return r.Get(ctx, messagesapp.MessageRef{
		WorkspaceID: params.WorkspaceID,
		ChannelID:   params.ChannelID,
		MessageID:   params.MessageID,
		ActorUserID: params.ActorUserID,
	})
}

func (r *Repository) replaceMentions(ctx context.Context, exec commandExecutor, workspaceID string, channelID string, messageID string, userIDs []string) error {
	if _, err := exec.Exec(ctx, `
DELETE FROM message_mentions
WHERE workspace_id = $1::uuid AND message_id = $2::uuid
`, workspaceID, messageID); err != nil {
		return err
	}

	for _, userID := range userIDs {
		command, err := exec.Exec(ctx, `
INSERT INTO message_mentions (workspace_id, message_id, mentioned_user_id)
SELECT $1::uuid, $2::uuid, cm.user_id
FROM channel_members cm
WHERE cm.channel_id = $3::uuid
  AND cm.user_id = $4::uuid
  AND cm.status IN ('active', 'muted')
ON CONFLICT DO NOTHING
`, workspaceID, messageID, channelID, userID)
		if err != nil {
			return err
		}
		if command.RowsAffected() == 0 {
			return messagesdomain.ErrMentionNotFound
		}
	}
	return nil
}

func (r *Repository) hydrateMessages(ctx context.Context, messages []messagesdomain.Message, actorUserID string) ([]messagesdomain.Message, error) {
	if len(messages) == 0 {
		return messages, nil
	}

	messageIDs := make([]pgtype.UUID, 0, len(messages))
	for _, message := range messages {
		var messageID pgtype.UUID
		if err := messageID.Scan(message.ID); err != nil {
			return nil, err
		}
		messageIDs = append(messageIDs, messageID)
	}

	mentionsByMessageID, err := r.messageMentionsBatch(ctx, messages[0].WorkspaceID, messageIDs)
	if err != nil {
		return nil, err
	}
	reactionsByMessageID, err := r.messageReactionsBatch(ctx, messages[0].WorkspaceID, messageIDs, actorUserID)
	if err != nil {
		return nil, err
	}

	for index := range messages {
		messages[index].Mentions = mentionsByMessageID[messages[index].ID]
		if messages[index].Mentions == nil {
			messages[index].Mentions = []string{}
		}
		messages[index].Reactions = reactionsByMessageID[messages[index].ID]
		if messages[index].Reactions == nil {
			messages[index].Reactions = []messagesdomain.ReactionSummary{}
		}
	}
	return messages, nil
}

func (r *Repository) messageMentionsBatch(ctx context.Context, workspaceID string, messageIDs []pgtype.UUID) (map[string][]string, error) {
	rows, err := r.pool.Query(ctx, `
SELECT message_id::text, mentioned_user_id::text
FROM message_mentions
WHERE workspace_id = $1::uuid
  AND message_id = ANY($2::uuid[])
ORDER BY message_id, mentioned_user_id
`, workspaceID, messageIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]string, len(messageIDs))
	for rows.Next() {
		var messageID string
		var userID string
		if err := rows.Scan(&messageID, &userID); err != nil {
			return nil, err
		}
		result[messageID] = append(result[messageID], userID)
	}
	return result, rows.Err()
}

func (r *Repository) messageReactionsBatch(ctx context.Context, workspaceID string, messageIDs []pgtype.UUID, actorUserID string) (map[string][]messagesdomain.ReactionSummary, error) {
	rows, err := r.pool.Query(ctx, `
SELECT message_id::text,
       emoji,
       count(*)::int,
       COALESCE(bool_or(user_id = NULLIF($3, '')::uuid), false)
FROM message_reactions
WHERE workspace_id = $1::uuid
  AND message_id = ANY($2::uuid[])
GROUP BY message_id, emoji
ORDER BY message_id, emoji
`, workspaceID, messageIDs, actorUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]messagesdomain.ReactionSummary, len(messageIDs))
	for rows.Next() {
		var messageID string
		var reaction messagesdomain.ReactionSummary
		if err := rows.Scan(&messageID, &reaction.Emoji, &reaction.Count, &reaction.ReactedByMe); err != nil {
			return nil, err
		}
		result[messageID] = append(result[messageID], reaction)
	}
	return result, rows.Err()
}

func (r *Repository) hydrateMessage(ctx context.Context, message messagesdomain.Message, actorUserID string) (messagesdomain.Message, error) {
	mentions, err := r.messageMentions(ctx, message.WorkspaceID, message.ID)
	if err != nil {
		return messagesdomain.Message{}, err
	}
	reactions, err := r.messageReactions(ctx, message.WorkspaceID, message.ID, actorUserID)
	if err != nil {
		return messagesdomain.Message{}, err
	}
	message.Mentions = mentions
	message.Reactions = reactions
	return message, nil
}

func (r *Repository) messageMentions(ctx context.Context, workspaceID string, messageID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `
SELECT mentioned_user_id::text
FROM message_mentions
WHERE workspace_id = $1::uuid AND message_id = $2::uuid
ORDER BY mentioned_user_id::text
`, workspaceID, messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	mentions := make([]string, 0)
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		mentions = append(mentions, userID)
	}
	return mentions, rows.Err()
}

func (r *Repository) messageReactions(ctx context.Context, workspaceID string, messageID string, actorUserID string) ([]messagesdomain.ReactionSummary, error) {
	rows, err := r.pool.Query(ctx, `
SELECT emoji,
       count(*)::int,
       COALESCE(bool_or(user_id = NULLIF($3, '')::uuid), false)
FROM message_reactions
WHERE workspace_id = $1::uuid AND message_id = $2::uuid
GROUP BY emoji
ORDER BY emoji
`, workspaceID, messageID, actorUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reactions := make([]messagesdomain.ReactionSummary, 0)
	for rows.Next() {
		var reaction messagesdomain.ReactionSummary
		if err := rows.Scan(&reaction.Emoji, &reaction.Count, &reaction.ReactedByMe); err != nil {
			return nil, err
		}
		reactions = append(reactions, reaction)
	}
	return reactions, rows.Err()
}

func upsertSearchDocument(ctx context.Context, exec commandExecutor, message messagesdomain.Message) error {
	metadata, err := json.Marshal(map[string]any{
		"channel_id": message.ChannelID,
		"sender_id":  message.SenderID,
		"kind":       message.Kind,
	})
	if err != nil {
		return err
	}
	_, err = exec.Exec(ctx, `
INSERT INTO search_documents (workspace_id, source_type, source_id, title, body, metadata)
VALUES ($1::uuid, 'message', $2::uuid, '', $3, $4::jsonb)
ON CONFLICT (workspace_id, source_type, source_id)
DO UPDATE SET body = EXCLUDED.body,
              metadata = EXCLUDED.metadata
`, message.WorkspaceID, message.ID, message.Body, string(metadata))
	return err
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

type rowScanner interface {
	Scan(dest ...any) error
}

func scanMessages(rows pgx.Rows) ([]messagesdomain.Message, error) {
	messages := make([]messagesdomain.Message, 0)
	for rows.Next() {
		message, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func scanMessage(row rowScanner) (messagesdomain.Message, error) {
	var message messagesdomain.Message
	var senderID sql.NullString
	var parentID sql.NullString
	var threadRootID sql.NullString
	var metadata string
	var editedAt sql.NullTime
	var deletedAt sql.NullTime

	if err := row.Scan(
		&message.ID,
		&message.WorkspaceID,
		&message.ChannelID,
		&senderID,
		&parentID,
		&threadRootID,
		&message.Kind,
		&message.Body,
		&metadata,
		&editedAt,
		&deletedAt,
		&message.CreatedAt,
		&message.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return messagesdomain.Message{}, messagesdomain.ErrMessageNotFound
		}
		return messagesdomain.Message{}, err
	}

	message.SenderID = nullStringPtr(senderID)
	message.ParentID = nullStringPtr(parentID)
	message.ThreadRootID = nullStringPtr(threadRootID)
	message.Metadata = []byte(metadata)
	message.EditedAt = nullTimePtr(editedAt)
	message.DeletedAt = nullTimePtr(deletedAt)
	return message, nil
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
