package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	orderapp "github.com/duclamdev/application-chat/backend/internal/modules/order/application"
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

func (r *Repository) WorkspaceSupportsOrderBot(ctx context.Context, workspaceID string) (bool, error) {
	var allowed bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM workspaces w
    JOIN zones z ON z.id = w.zone_id
    WHERE w.id = $1::uuid
      AND w.status = 'active'
      AND w.deleted_at IS NULL
      AND z.kind = 'vpsttt_internal'
      AND z.status = 'active'
)
`, workspaceID).Scan(&allowed)
	return allowed, err
}

func (r *Repository) UserEmailByID(ctx context.Context, userID string) (string, error) {
	var email string
	err := r.pool.QueryRow(ctx, `
SELECT email
FROM users
WHERE id = $1::uuid
  AND status = 'active'
  AND deleted_at IS NULL
`, userID).Scan(&email)
	if err != nil {
		return "", err
	}
	return email, nil
}

func (r *Repository) ChannelByID(ctx context.Context, workspaceID string, channelID string) (orderapp.ChannelDTO, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id::text, workspace_id::text, COALESCE(settings->>'bot_source_slug', slug::text), name
FROM channels
WHERE workspace_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'
  AND deleted_at IS NULL
`, workspaceID, channelID)
	var channel orderapp.ChannelDTO
	if err := row.Scan(&channel.ID, &channel.WorkspaceID, &channel.Slug, &channel.Name); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return orderapp.ChannelDTO{}, orderapp.ErrOrderChannelNotFound
		}
		return orderapp.ChannelDTO{}, err
	}
	return channel, nil
}

func (r *Repository) SendBotMessage(ctx context.Context, params orderapp.SendBotMessageParams) (orderapp.BotMessageDTO, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return orderapp.BotMessageDTO{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	row := tx.QueryRow(ctx, `
WITH target AS (
    SELECT c.workspace_id, c.id AS channel_id, b.id AS bot_id
    FROM channels c
    JOIN workspaces w
      ON w.id = c.workspace_id
     AND w.status = 'active'
     AND w.deleted_at IS NULL
    JOIN zones z
      ON z.id = w.zone_id
     AND z.kind = 'vpsttt_internal'
     AND z.status = 'active'
    JOIN bots b
      ON b.workspace_id = c.workspace_id
     AND b.slug::text = $2
     AND b.status = 'active'
     AND b.deleted_at IS NULL
    WHERE c.workspace_id = $1::uuid
      AND c.status = 'active'
      AND c.deleted_at IS NULL
      AND (
          c.id = NULLIF($4, '')::uuid
          OR (NULLIF($4, '') IS NULL AND c.slug::text = $5)
      )
      AND EXISTS (
          SELECT 1
          FROM bot_installations bi
          WHERE bi.bot_id = b.id
            AND bi.workspace_id = c.workspace_id
            AND bi.status = 'active'
            AND (
                bi.channel_id IS NULL
                OR bi.channel_id = c.id
                OR bi.channel_id::text = c.settings->>'bot_source_channel_id'
            )
      )
    LIMIT 1
), inserted AS (
    INSERT INTO messages (workspace_id, channel_id, sender_id, kind, body, metadata)
    SELECT target.workspace_id,
           target.channel_id,
           NULL,
           'bot',
           $3,
           $6::jsonb || jsonb_build_object('bot_id', target.bot_id::text, 'bot_slug', $2)
    FROM target
    RETURNING id::text, workspace_id::text, channel_id::text, kind, body, metadata::text, created_at
)
SELECT inserted.id, inserted.workspace_id, inserted.channel_id, target.bot_id::text,
       inserted.kind, inserted.body, inserted.metadata, inserted.created_at
FROM inserted
JOIN target ON target.channel_id::text = inserted.channel_id
`, params.WorkspaceID, params.BotSlug, params.Body, params.ChannelID, params.ChannelSlug, string(params.Metadata))

	message, err := scanBotMessage(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return orderapp.BotMessageDTO{}, orderapp.ErrOrderBotTargetNotFound
		}
		return orderapp.BotMessageDTO{}, err
	}
	if err := upsertSearchDocument(ctx, tx, message); err != nil {
		return orderapp.BotMessageDTO{}, err
	}
	if err := insertOutbox(ctx, tx, "message", message.ID, "MessageCreated", map[string]any{
		"workspace_id":       message.WorkspaceID,
		"channel_id":         message.ChannelID,
		"message_id":         message.ID,
		"sender_id":          "",
		"bot_id":             message.BotID,
		"mentioned_user_ids": []string{},
	}); err != nil {
		return orderapp.BotMessageDTO{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return orderapp.BotMessageDTO{}, err
	}
	return message, nil
}

type commandExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func upsertSearchDocument(ctx context.Context, exec commandExecutor, message orderapp.BotMessageDTO) error {
	metadata, err := json.Marshal(map[string]any{
		"channel_id": message.ChannelID,
		"bot_id":     message.BotID,
		"kind":       message.Kind,
		"source":     "vpsttt_order",
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

func scanBotMessage(row rowScanner) (orderapp.BotMessageDTO, error) {
	var message orderapp.BotMessageDTO
	var metadata string
	var createdAt time.Time
	if err := row.Scan(
		&message.ID,
		&message.WorkspaceID,
		&message.ChannelID,
		&message.BotID,
		&message.Kind,
		&message.Body,
		&metadata,
		&createdAt,
	); err != nil {
		return orderapp.BotMessageDTO{}, err
	}
	message.Metadata = json.RawMessage(metadata)
	message.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	return message, nil
}
