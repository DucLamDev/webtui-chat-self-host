package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	botsapp "github.com/duclamdev/application-chat/backend/internal/modules/bots/application"
	botsdomain "github.com/duclamdev/application-chat/backend/internal/modules/bots/domain"
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

func (r *Repository) CreateBot(ctx context.Context, params botsapp.CreateBotParams) (botsdomain.Bot, error) {
	row := r.pool.QueryRow(ctx, `
INSERT INTO bots (workspace_id, slug, name, description, avatar_url, created_by, settings)
VALUES ($1::uuid, $2, $3, NULLIF($4, ''), NULLIF($5, ''), $6::uuid, $7::jsonb)
RETURNING id::text, workspace_id::text, slug::text, name, description, avatar_url, status,
          created_by::text, settings::text, created_at, updated_at
`, params.WorkspaceID, params.Slug, params.Name, params.Description, params.AvatarURL, params.CreatedBy, string(params.Settings))
	bot, err := scanBot(row)
	if isUniqueViolation(err) {
		return botsdomain.Bot{}, botsdomain.ErrBotAlreadyExists
	}
	return bot, err
}

func (r *Repository) ListBots(ctx context.Context, workspaceID string) ([]botsdomain.Bot, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, workspace_id::text, slug::text, name, description, avatar_url, status,
       created_by::text, settings::text, created_at, updated_at
FROM bots
WHERE workspace_id = $1::uuid AND deleted_at IS NULL
ORDER BY name, slug
`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bots := make([]botsdomain.Bot, 0)
	for rows.Next() {
		bot, err := scanBot(rows)
		if err != nil {
			return nil, err
		}
		bots = append(bots, bot)
	}
	return bots, rows.Err()
}

func (r *Repository) InstallBot(ctx context.Context, params botsapp.InstallBotParams) (botsdomain.Installation, error) {
	row := r.pool.QueryRow(ctx, `
INSERT INTO bot_installations (bot_id, workspace_id, channel_id, config)
SELECT b.id, b.workspace_id, NULLIF($3, '')::uuid, $4::jsonb
FROM bots b
WHERE b.id = $2::uuid
  AND b.workspace_id = $1::uuid
  AND b.status = 'active'
  AND b.deleted_at IS NULL
  AND (
      $3 = ''
      OR EXISTS (
          SELECT 1
          FROM channels c
          WHERE c.id = $3::uuid
            AND c.workspace_id = b.workspace_id
            AND c.deleted_at IS NULL
            AND c.status = 'active'
      )
  )
RETURNING id::text, bot_id::text, workspace_id::text, channel_id::text, status, config::text, created_at, updated_at
`, params.WorkspaceID, params.BotID, params.ChannelID, string(params.Config))
	installation, err := scanInstallation(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return botsdomain.Installation{}, botsdomain.ErrBotNotFound
	}
	if isUniqueViolation(err) {
		return botsdomain.Installation{}, botsdomain.ErrBotAlreadyInstalled
	}
	return installation, err
}

func (r *Repository) ListInstallations(ctx context.Context, workspaceID string, botID string) ([]botsdomain.Installation, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, bot_id::text, workspace_id::text, channel_id::text, status, config::text, created_at, updated_at
FROM bot_installations
WHERE workspace_id = $1::uuid
  AND ($2 = '' OR bot_id = NULLIF($2, '')::uuid)
ORDER BY created_at DESC
`, workspaceID, botID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	installations := make([]botsdomain.Installation, 0)
	for rows.Next() {
		installation, err := scanInstallation(rows)
		if err != nil {
			return nil, err
		}
		installations = append(installations, installation)
	}
	return installations, rows.Err()
}

func (r *Repository) SendBotMessage(ctx context.Context, params botsapp.SendBotMessageParams) (botsdomain.BotMessage, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return botsdomain.BotMessage{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	metadata, err := mergeMetadata(params.Metadata, map[string]any{"bot_id": params.BotID})
	if err != nil {
		return botsdomain.BotMessage{}, err
	}

	row := tx.QueryRow(ctx, `
INSERT INTO messages (workspace_id, channel_id, sender_id, kind, body, metadata)
SELECT c.workspace_id, c.id, NULL, 'bot', $4, $5::jsonb
FROM channels c
WHERE c.workspace_id = $1::uuid
  AND c.id = $3::uuid
  AND c.deleted_at IS NULL
  AND c.status = 'active'
  AND EXISTS (
      SELECT 1
      FROM bots b
      WHERE b.id = $2::uuid
        AND b.workspace_id = c.workspace_id
        AND b.status = 'active'
        AND b.deleted_at IS NULL
  )
  AND EXISTS (
      SELECT 1
      FROM bot_installations bi
      WHERE bi.bot_id = $2::uuid
        AND bi.workspace_id = c.workspace_id
        AND bi.status = 'active'
        AND (bi.channel_id IS NULL OR bi.channel_id = c.id)
  )
RETURNING id::text, workspace_id::text, channel_id::text, kind, body, metadata::text, created_at
`, params.WorkspaceID, params.BotID, params.ChannelID, params.Body, string(metadata))
	message, err := scanBotMessage(row, params.BotID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return botsdomain.BotMessage{}, botsdomain.ErrBotNotInstalled
		}
		return botsdomain.BotMessage{}, err
	}

	if err := upsertSearchDocument(ctx, tx, message); err != nil {
		return botsdomain.BotMessage{}, err
	}
	if err := insertOutbox(ctx, tx, "message", message.ID, "MessageCreated", map[string]any{
		"workspace_id":       message.WorkspaceID,
		"channel_id":         message.ChannelID,
		"message_id":         message.ID,
		"sender_id":          "",
		"bot_id":             params.BotID,
		"mentioned_user_ids": []string{},
	}); err != nil {
		return botsdomain.BotMessage{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return botsdomain.BotMessage{}, err
	}
	return message, nil
}

func (r *Repository) GetAIConfig(ctx context.Context, workspaceID string, botID string) (botsdomain.AIConfig, error) {
	row := r.pool.QueryRow(ctx, `
SELECT workspace_id::text, bot_id::text, provider, model, secret_ref, settings::text,
       created_by::text, updated_by::text, created_at, updated_at
FROM bot_ai_configs
WHERE workspace_id = $1::uuid AND bot_id = $2::uuid
`, workspaceID, botID)
	return scanAIConfig(row)
}

func (r *Repository) UpsertAIConfig(ctx context.Context, params botsapp.AIConfigParams) (botsdomain.AIConfig, error) {
	row := r.pool.QueryRow(ctx, `
WITH bot AS (
    SELECT id, workspace_id
    FROM bots
    WHERE workspace_id = $1::uuid
      AND id = $2::uuid
      AND deleted_at IS NULL
)
INSERT INTO bot_ai_configs (workspace_id, bot_id, provider, model, secret_ref, settings, created_by, updated_by)
SELECT workspace_id, id, $3, $4, NULLIF($5, ''), $6::jsonb, $7::uuid, $7::uuid
FROM bot
ON CONFLICT (workspace_id, bot_id) DO UPDATE SET
    provider = EXCLUDED.provider,
    model = EXCLUDED.model,
    secret_ref = EXCLUDED.secret_ref,
    settings = EXCLUDED.settings,
    updated_by = EXCLUDED.updated_by
RETURNING workspace_id::text, bot_id::text, provider, model, secret_ref, settings::text,
          created_by::text, updated_by::text, created_at, updated_at
`, params.WorkspaceID, params.BotID, params.Provider, params.Model, params.SecretRef, string(params.Settings), params.ActorUserID)
	return scanAIConfig(row)
}

func (r *Repository) ListFlows(ctx context.Context, workspaceID string, botID string) ([]botsdomain.Flow, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, workspace_id::text, bot_id::text, version, status, name, prompt,
       trigger_config::text, tool_config::text, knowledge_config::text,
       created_by::text, updated_by::text, published_at, created_at, updated_at
FROM bot_flows
WHERE workspace_id = $1::uuid AND bot_id = $2::uuid
ORDER BY version DESC, created_at DESC
`, workspaceID, botID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	flows := make([]botsdomain.Flow, 0)
	for rows.Next() {
		flow, err := scanFlow(rows)
		if err != nil {
			return nil, err
		}
		flows = append(flows, flow)
	}
	return flows, rows.Err()
}

func (r *Repository) CreateFlow(ctx context.Context, params botsapp.FlowParams) (botsdomain.Flow, error) {
	row := r.pool.QueryRow(ctx, `
WITH next_version AS (
    SELECT COALESCE(MAX(version), 0) + 1 AS value
    FROM bot_flows
    WHERE workspace_id = $1::uuid AND bot_id = $2::uuid
),
bot AS (
    SELECT id, workspace_id
    FROM bots
    WHERE workspace_id = $1::uuid
      AND id = $2::uuid
      AND deleted_at IS NULL
)
INSERT INTO bot_flows (workspace_id, bot_id, version, name, prompt, trigger_config, tool_config, knowledge_config, created_by, updated_by)
SELECT bot.workspace_id, bot.id, next_version.value, $3, $4, $5::jsonb, $6::jsonb, $7::jsonb, $8::uuid, $8::uuid
FROM bot, next_version
RETURNING id::text, workspace_id::text, bot_id::text, version, status, name, prompt,
          trigger_config::text, tool_config::text, knowledge_config::text,
          created_by::text, updated_by::text, published_at, created_at, updated_at
`, params.WorkspaceID, params.BotID, params.Name, params.Prompt, string(params.TriggerConfig), string(params.ToolConfig), string(params.KnowledgeConfig), params.ActorUserID)
	return scanFlow(row)
}

func (r *Repository) UpdateFlow(ctx context.Context, params botsapp.FlowParams) (botsdomain.Flow, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE bot_flows
SET name = $4,
    prompt = $5,
    trigger_config = $6::jsonb,
    tool_config = $7::jsonb,
    knowledge_config = $8::jsonb,
    updated_by = $9::uuid
WHERE workspace_id = $1::uuid
  AND bot_id = $2::uuid
  AND id = $3::uuid
  AND status = 'draft'
RETURNING id::text, workspace_id::text, bot_id::text, version, status, name, prompt,
          trigger_config::text, tool_config::text, knowledge_config::text,
          created_by::text, updated_by::text, published_at, created_at, updated_at
`, params.WorkspaceID, params.BotID, params.FlowID, params.Name, params.Prompt, string(params.TriggerConfig), string(params.ToolConfig), string(params.KnowledgeConfig), params.ActorUserID)
	return scanFlow(row)
}

func (r *Repository) PublishFlow(ctx context.Context, params botsapp.PublishFlowParams) (botsdomain.Flow, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return botsdomain.Flow{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	if _, err := tx.Exec(ctx, `
UPDATE bot_flows
SET status = 'archived'
WHERE workspace_id = $1::uuid
  AND bot_id = $2::uuid
  AND status = 'published'
`, params.WorkspaceID, params.BotID); err != nil {
		return botsdomain.Flow{}, err
	}
	row := tx.QueryRow(ctx, `
UPDATE bot_flows
SET status = 'published',
    published_at = now(),
    updated_by = $4::uuid
WHERE workspace_id = $1::uuid
  AND bot_id = $2::uuid
  AND id = $3::uuid
RETURNING id::text, workspace_id::text, bot_id::text, version, status, name, prompt,
          trigger_config::text, tool_config::text, knowledge_config::text,
          created_by::text, updated_by::text, published_at, created_at, updated_at
`, params.WorkspaceID, params.BotID, params.FlowID, params.ActorUserID)
	flow, err := scanFlow(row)
	if err != nil {
		return botsdomain.Flow{}, err
	}
	return flow, tx.Commit(ctx)
}

func (r *Repository) TestFlow(ctx context.Context, params botsapp.TestFlowParams) (botsdomain.FlowRun, error) {
	transcript, err := json.Marshal(map[string]any{
		"message":    "Flow đã được nhận để kiểm thử. Runtime AI thật sẽ xử lý ở worker khi được cấu hình.",
		"tool_calls": []any{},
	})
	if err != nil {
		return botsdomain.FlowRun{}, err
	}
	row := r.pool.QueryRow(ctx, `
INSERT INTO bot_flow_runs (workspace_id, bot_id, flow_id, input, transcript, status, created_by)
SELECT workspace_id, bot_id, id, $4::jsonb, $5::jsonb, 'success', $6::uuid
FROM bot_flows
WHERE workspace_id = $1::uuid
  AND bot_id = $2::uuid
  AND id = $3::uuid
RETURNING id::text, workspace_id::text, bot_id::text, flow_id::text, input::text, transcript::text,
          status, error, created_by::text, created_at
`, params.WorkspaceID, params.BotID, params.FlowID, string(params.Input), string(transcript), params.ActorUserID)
	return scanFlowRun(row)
}

type commandExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

func upsertSearchDocument(ctx context.Context, exec commandExecutor, message botsdomain.BotMessage) error {
	metadata, err := json.Marshal(map[string]any{
		"channel_id": message.ChannelID,
		"bot_id":     message.BotID,
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

func scanBot(row rowScanner) (botsdomain.Bot, error) {
	var bot botsdomain.Bot
	var description sql.NullString
	var avatarURL sql.NullString
	var createdBy sql.NullString
	var settings string
	if err := row.Scan(
		&bot.ID,
		&bot.WorkspaceID,
		&bot.Slug,
		&bot.Name,
		&description,
		&avatarURL,
		&bot.Status,
		&createdBy,
		&settings,
		&bot.CreatedAt,
		&bot.UpdatedAt,
	); err != nil {
		return botsdomain.Bot{}, err
	}
	bot.Description = nullStringPtr(description)
	bot.AvatarURL = nullStringPtr(avatarURL)
	bot.CreatedBy = nullStringPtr(createdBy)
	bot.Settings = []byte(settings)
	return bot, nil
}

func scanInstallation(row rowScanner) (botsdomain.Installation, error) {
	var installation botsdomain.Installation
	var channelID sql.NullString
	var config string
	if err := row.Scan(
		&installation.ID,
		&installation.BotID,
		&installation.WorkspaceID,
		&channelID,
		&installation.Status,
		&config,
		&installation.CreatedAt,
		&installation.UpdatedAt,
	); err != nil {
		return botsdomain.Installation{}, err
	}
	installation.ChannelID = nullStringPtr(channelID)
	installation.Config = []byte(config)
	return installation, nil
}

func scanBotMessage(row rowScanner, botID string) (botsdomain.BotMessage, error) {
	var message botsdomain.BotMessage
	var metadata string
	if err := row.Scan(
		&message.ID,
		&message.WorkspaceID,
		&message.ChannelID,
		&message.Kind,
		&message.Body,
		&metadata,
		&message.CreatedAt,
	); err != nil {
		return botsdomain.BotMessage{}, err
	}
	message.BotID = botID
	message.Metadata = []byte(metadata)
	return message, nil
}

func scanAIConfig(row rowScanner) (botsdomain.AIConfig, error) {
	var config botsdomain.AIConfig
	var secretRef sql.NullString
	var createdBy sql.NullString
	var updatedBy sql.NullString
	var settings string
	if err := row.Scan(
		&config.WorkspaceID,
		&config.BotID,
		&config.Provider,
		&config.Model,
		&secretRef,
		&settings,
		&createdBy,
		&updatedBy,
		&config.CreatedAt,
		&config.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return botsdomain.AIConfig{}, botsdomain.ErrBotNotFound
		}
		return botsdomain.AIConfig{}, err
	}
	config.SecretRef = nullStringPtr(secretRef)
	config.Settings = []byte(settings)
	config.CreatedBy = nullStringPtr(createdBy)
	config.UpdatedBy = nullStringPtr(updatedBy)
	return config, nil
}

func scanFlow(row rowScanner) (botsdomain.Flow, error) {
	var flow botsdomain.Flow
	var triggerConfig string
	var toolConfig string
	var knowledgeConfig string
	var createdBy sql.NullString
	var updatedBy sql.NullString
	var publishedAt sql.NullTime
	if err := row.Scan(
		&flow.ID,
		&flow.WorkspaceID,
		&flow.BotID,
		&flow.Version,
		&flow.Status,
		&flow.Name,
		&flow.Prompt,
		&triggerConfig,
		&toolConfig,
		&knowledgeConfig,
		&createdBy,
		&updatedBy,
		&publishedAt,
		&flow.CreatedAt,
		&flow.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return botsdomain.Flow{}, botsdomain.ErrBotNotFound
		}
		return botsdomain.Flow{}, err
	}
	flow.TriggerConfig = []byte(triggerConfig)
	flow.ToolConfig = []byte(toolConfig)
	flow.KnowledgeConfig = []byte(knowledgeConfig)
	flow.CreatedBy = nullStringPtr(createdBy)
	flow.UpdatedBy = nullStringPtr(updatedBy)
	if publishedAt.Valid {
		flow.PublishedAt = &publishedAt.Time
	}
	return flow, nil
}

func scanFlowRun(row rowScanner) (botsdomain.FlowRun, error) {
	var run botsdomain.FlowRun
	var input string
	var transcript string
	var errorMessage sql.NullString
	var createdBy sql.NullString
	if err := row.Scan(
		&run.ID,
		&run.WorkspaceID,
		&run.BotID,
		&run.FlowID,
		&input,
		&transcript,
		&run.Status,
		&errorMessage,
		&createdBy,
		&run.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return botsdomain.FlowRun{}, botsdomain.ErrBotNotFound
		}
		return botsdomain.FlowRun{}, err
	}
	run.Input = []byte(input)
	run.Transcript = []byte(transcript)
	run.Error = nullStringPtr(errorMessage)
	run.CreatedBy = nullStringPtr(createdBy)
	return run, nil
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
