package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	webhooksapp "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/application"
	webhooksdomain "github.com/duclamdev/application-chat/backend/internal/modules/webhooks/domain"
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

func (r *Repository) CreateIncoming(ctx context.Context, params webhooksapp.CreateIncomingParams) (webhooksdomain.IncomingWebhook, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return webhooksdomain.IncomingWebhook{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	if err := ensureWebhookQuota(ctx, tx, params.WorkspaceID); err != nil {
		return webhooksdomain.IncomingWebhook{}, err
	}
	row := tx.QueryRow(ctx, `
INSERT INTO incoming_webhooks (workspace_id, channel_id, name, secret_hash, created_by)
SELECT $1::uuid, NULLIF($2, '')::uuid, $3, $4, $5::uuid
WHERE $2 = ''
   OR EXISTS (
       SELECT 1
       FROM channels
       WHERE workspace_id = $1::uuid
         AND id = $2::uuid
         AND status = 'active'
         AND deleted_at IS NULL
   )
RETURNING id::text, workspace_id::text, channel_id::text, name, status, created_by::text,
          created_at, updated_at, last_used_at
`, params.WorkspaceID, params.ChannelID, params.Name, params.SecretHash, params.CreatedBy)
	webhook, err := scanIncoming(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return webhooksdomain.IncomingWebhook{}, webhooksdomain.ErrIncomingWebhookNotFound
	}
	if err != nil {
		return webhooksdomain.IncomingWebhook{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return webhooksdomain.IncomingWebhook{}, err
	}
	return webhook, nil
}

func (r *Repository) ListIncoming(ctx context.Context, workspaceID string) ([]webhooksdomain.IncomingWebhook, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, workspace_id::text, channel_id::text, name, status, created_by::text,
       created_at, updated_at, last_used_at
FROM incoming_webhooks
WHERE workspace_id = $1::uuid
ORDER BY created_at DESC
`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	webhooks := make([]webhooksdomain.IncomingWebhook, 0)
	for rows.Next() {
		webhook, err := scanIncoming(rows)
		if err != nil {
			return nil, err
		}
		webhooks = append(webhooks, webhook)
	}
	return webhooks, rows.Err()
}

func (r *Repository) UpdateIncoming(ctx context.Context, params webhooksapp.UpdateIncomingParams) (webhooksdomain.IncomingWebhook, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE incoming_webhooks iw
SET name = COALESCE($3::text, iw.name),
    status = COALESCE($4::text, iw.status),
    channel_id = CASE WHEN $5::text IS NULL THEN iw.channel_id ELSE NULLIF($5::text, '')::uuid END
WHERE iw.workspace_id = $1::uuid
  AND iw.id = $2::uuid
  AND (
      $5::text IS NULL
      OR $5::text = ''
      OR EXISTS (
          SELECT 1
          FROM channels c
          WHERE c.workspace_id = iw.workspace_id
            AND c.id = NULLIF($5::text, '')::uuid
            AND c.status = 'active'
            AND c.deleted_at IS NULL
      )
  )
RETURNING id::text, workspace_id::text, channel_id::text, name, status, created_by::text,
          created_at, updated_at, last_used_at
`, params.WorkspaceID, params.WebhookID, nullableString(params.Name), nullableString(params.Status), nullableString(params.ChannelID))
	webhook, err := scanIncoming(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return webhooksdomain.IncomingWebhook{}, webhooksdomain.ErrIncomingWebhookNotFound
	}
	return webhook, err
}

func (r *Repository) DeleteIncoming(ctx context.Context, workspaceID string, incomingWebhookID string) error {
	tag, err := r.pool.Exec(ctx, `
DELETE FROM incoming_webhooks
WHERE workspace_id = $1::uuid
  AND id = $2::uuid
`, workspaceID, incomingWebhookID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return webhooksdomain.ErrIncomingWebhookNotFound
	}
	return nil
}

func (r *Repository) CreateOutgoing(ctx context.Context, params webhooksapp.CreateOutgoingParams) (webhooksdomain.OutgoingWebhook, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return webhooksdomain.OutgoingWebhook{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	if err := ensureWebhookQuota(ctx, tx, params.WorkspaceID); err != nil {
		return webhooksdomain.OutgoingWebhook{}, err
	}
	row := tx.QueryRow(ctx, `
INSERT INTO outgoing_webhooks (workspace_id, name, target_url, signing_secret_encrypted, event_types, created_by)
VALUES ($1::uuid, $2, $3, $4, $5, $6::uuid)
RETURNING id::text, workspace_id::text, name, target_url, signing_secret_encrypted, event_types, status,
          created_by::text, created_at, updated_at
`, params.WorkspaceID, params.Name, params.TargetURL, params.SigningSecretEncrypted, params.EventTypes, params.CreatedBy)
	webhook, err := scanOutgoing(row)
	if err != nil {
		return webhooksdomain.OutgoingWebhook{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return webhooksdomain.OutgoingWebhook{}, err
	}
	return webhook, nil
}

func (r *Repository) ListOutgoing(ctx context.Context, workspaceID string) ([]webhooksdomain.OutgoingWebhook, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, workspace_id::text, name, target_url, signing_secret_encrypted, event_types, status,
       created_by::text, created_at, updated_at
FROM outgoing_webhooks
WHERE workspace_id = $1::uuid
ORDER BY created_at DESC
`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	webhooks := make([]webhooksdomain.OutgoingWebhook, 0)
	for rows.Next() {
		webhook, err := scanOutgoing(rows)
		if err != nil {
			return nil, err
		}
		webhooks = append(webhooks, webhook)
	}
	return webhooks, rows.Err()
}

func (r *Repository) UpdateOutgoing(ctx context.Context, params webhooksapp.UpdateOutgoingParams) (webhooksdomain.OutgoingWebhook, error) {
	eventTypes := []string{}
	hasEventTypes := params.EventTypes != nil
	if params.EventTypes != nil {
		eventTypes = *params.EventTypes
	}
	row := r.pool.QueryRow(ctx, `
UPDATE outgoing_webhooks ow
SET name = COALESCE($3::text, ow.name),
    target_url = COALESCE($4::text, ow.target_url),
    status = COALESCE($5::text, ow.status),
    event_types = CASE WHEN $6::boolean THEN $7::text[] ELSE ow.event_types END
WHERE ow.workspace_id = $1::uuid
  AND ow.id = $2::uuid
RETURNING id::text, workspace_id::text, name, target_url, signing_secret_encrypted, event_types, status,
          created_by::text, created_at, updated_at
`, params.WorkspaceID, params.WebhookID, nullableString(params.Name), nullableString(params.TargetURL), nullableString(params.Status), hasEventTypes, eventTypes)
	webhook, err := scanOutgoing(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return webhooksdomain.OutgoingWebhook{}, webhooksdomain.ErrOutgoingWebhookNotFound
	}
	return webhook, err
}

func (r *Repository) DeleteOutgoing(ctx context.Context, workspaceID string, outgoingWebhookID string) error {
	tag, err := r.pool.Exec(ctx, `
DELETE FROM outgoing_webhooks
WHERE workspace_id = $1::uuid
  AND id = $2::uuid
`, workspaceID, outgoingWebhookID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return webhooksdomain.ErrOutgoingWebhookNotFound
	}
	return nil
}

func (r *Repository) ListDeliveries(ctx context.Context, workspaceID string, outgoingWebhookID string, limit int) ([]webhooksdomain.Delivery, error) {
	rows, err := r.pool.Query(ctx, `
SELECT d.id::text, d.outgoing_webhook_id::text, ow.target_url, ow.signing_secret_encrypted, d.event_id::text,
       d.event_type, d.request_body::text, d.response_status, d.response_body, d.status,
       d.attempt_count, d.next_attempt_at, d.delivered_at, d.created_at, d.updated_at
FROM webhook_deliveries d
JOIN outgoing_webhooks ow ON ow.id = d.outgoing_webhook_id
WHERE ow.workspace_id = $1::uuid
  AND ($2 = '' OR ow.id = NULLIF($2, '')::uuid)
ORDER BY d.created_at DESC
LIMIT $3
`, workspaceID, outgoingWebhookID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	deliveries := make([]webhooksdomain.Delivery, 0)
	for rows.Next() {
		delivery, err := scanDelivery(rows)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, delivery)
	}
	return deliveries, rows.Err()
}

func (r *Repository) CreateTestDelivery(ctx context.Context, params webhooksapp.TestDeliveryParams) (webhooksdomain.Delivery, error) {
	row := r.pool.QueryRow(ctx, `
WITH target AS (
    SELECT id, target_url, signing_secret_encrypted
    FROM outgoing_webhooks
    WHERE workspace_id = $1::uuid
      AND id = $2::uuid
),
inserted AS (
    INSERT INTO webhook_deliveries (outgoing_webhook_id, event_type, request_body)
    SELECT target.id, $3, $4::jsonb
    FROM target
    RETURNING id, outgoing_webhook_id, event_id, event_type, request_body::text,
              response_status, response_body, status, attempt_count, next_attempt_at,
              delivered_at, created_at, updated_at
)
SELECT inserted.id::text,
       inserted.outgoing_webhook_id::text,
       target.target_url,
       target.signing_secret_encrypted,
       inserted.event_id::text,
       inserted.event_type,
       inserted.request_body,
       inserted.response_status,
       inserted.response_body,
       inserted.status,
       inserted.attempt_count,
       inserted.next_attempt_at,
       inserted.delivered_at,
       inserted.created_at,
       inserted.updated_at
FROM inserted
JOIN target ON target.id = inserted.outgoing_webhook_id
`, params.WorkspaceID, params.OutgoingWebhookID, params.EventType, string(params.RequestBody))
	delivery, err := scanDelivery(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return webhooksdomain.Delivery{}, webhooksdomain.ErrOutgoingWebhookNotFound
	}
	return delivery, err
}

func (r *Repository) SendIncomingMessage(ctx context.Context, params webhooksapp.IncomingMessageParams) (webhooksdomain.IntegrationMessage, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return webhooksdomain.IntegrationMessage{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var webhookID string
	var workspaceID string
	var channelID string
	row := tx.QueryRow(ctx, `
SELECT iw.id::text, iw.workspace_id::text, COALESCE(NULLIF($3, '')::uuid, iw.channel_id)::text
FROM incoming_webhooks iw
JOIN workspaces workspace
  ON workspace.id = iw.workspace_id
 AND workspace.zone_id = $4::uuid
 AND workspace.status = 'active'
 AND workspace.deleted_at IS NULL
JOIN zones zone
  ON zone.id = workspace.zone_id
 AND zone.status = 'active'
 AND zone.deleted_at IS NULL
JOIN channels c
  ON c.workspace_id = iw.workspace_id
 AND c.id = COALESCE(NULLIF($3, '')::uuid, iw.channel_id)
 AND c.status = 'active'
 AND c.deleted_at IS NULL
WHERE iw.id = $1::uuid
  AND iw.secret_hash = $2
  AND iw.status = 'active'
`, params.WebhookID, params.SecretHash, params.ChannelID, params.ExpectedZoneID)
	if err := row.Scan(&webhookID, &workspaceID, &channelID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return webhooksdomain.IntegrationMessage{}, webhooksdomain.ErrIncomingWebhookNotFound
		}
		return webhooksdomain.IntegrationMessage{}, err
	}

	if _, err := tx.Exec(ctx, `UPDATE incoming_webhooks SET last_used_at = now() WHERE id = $1::uuid`, webhookID); err != nil {
		return webhooksdomain.IntegrationMessage{}, err
	}
	metadata, err := mergeMetadata(params.Metadata, map[string]any{
		"source":              "incoming_webhook",
		"incoming_webhook_id": webhookID,
	})
	if err != nil {
		return webhooksdomain.IntegrationMessage{}, err
	}
	message, err := insertIntegrationMessage(ctx, tx, webhooksapp.IntegrationMessageParams{
		WorkspaceID: workspaceID,
		ChannelID:   channelID,
		Kind:        "event",
		Body:        params.Body,
		Metadata:    metadata,
	})
	if err != nil {
		return webhooksdomain.IntegrationMessage{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return webhooksdomain.IntegrationMessage{}, err
	}
	return message, nil
}

func (r *Repository) SendIntegrationMessage(ctx context.Context, params webhooksapp.IntegrationMessageParams) (webhooksdomain.IntegrationMessage, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return webhooksdomain.IntegrationMessage{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	metadata, err := mergeMetadata(params.Metadata, map[string]any{"source": "api_token"})
	if err != nil {
		return webhooksdomain.IntegrationMessage{}, err
	}
	params.Metadata = metadata
	message, err := insertIntegrationMessage(ctx, tx, params)
	if err != nil {
		return webhooksdomain.IntegrationMessage{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return webhooksdomain.IntegrationMessage{}, err
	}
	return message, nil
}

func (r *Repository) CreateDeliveriesForEvent(ctx context.Context, params webhooksapp.OutboxDeliveryParams) (int, error) {
	rows, err := r.pool.Query(ctx, `
INSERT INTO webhook_deliveries (outgoing_webhook_id, event_id, event_type, request_body)
SELECT ow.id, $1::uuid, $2, $4::jsonb
FROM outgoing_webhooks ow
WHERE ow.workspace_id = $3::uuid
  AND ow.status = 'active'
  AND (
      cardinality(ow.event_types) = 0
      OR $2 = ANY(ow.event_types)
  )
  AND NOT EXISTS (
      SELECT 1
      FROM automation_installations installation
      WHERE installation.runtime_webhook_id = ow.id
        AND installation.deleted_at IS NULL
        AND COALESCE(installation.config->>'channel_slug', '') <> ''
        AND NOT EXISTS (
            SELECT 1
            FROM channels channel
            WHERE channel.workspace_id = ow.workspace_id
              AND channel.id::text = $4::jsonb->>'channel_id'
              AND channel.slug::text = installation.config->>'channel_slug'
              AND channel.status = 'active'
              AND channel.deleted_at IS NULL
        )
  )
  AND NOT EXISTS (
      SELECT 1
      FROM webhook_deliveries d
      WHERE d.outgoing_webhook_id = ow.id
        AND d.event_id = $1::uuid
  )
RETURNING id
`, params.EventID, params.EventType, params.WorkspaceID, string(params.RequestBody))
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		count++
	}
	return count, rows.Err()
}

func ensureWebhookQuota(ctx context.Context, tx pgx.Tx, workspaceID string) error {
	var enforcementMode string
	var maximum int
	var current int
	err := tx.QueryRow(ctx, `
SELECT quota.enforcement_mode,
       quota.max_webhooks,
       (
           SELECT count(*)
           FROM (
               SELECT incoming.id
               FROM incoming_webhooks incoming
               JOIN workspaces item_workspace ON item_workspace.id = incoming.workspace_id
               WHERE item_workspace.zone_id = quota.zone_id
                 AND item_workspace.deleted_at IS NULL
               UNION ALL
               SELECT outgoing.id
               FROM outgoing_webhooks outgoing
               JOIN workspaces item_workspace ON item_workspace.id = outgoing.workspace_id
               WHERE item_workspace.zone_id = quota.zone_id
                 AND item_workspace.deleted_at IS NULL
           ) webhook
       )
FROM workspaces workspace
JOIN zone_quotas quota ON quota.zone_id = workspace.zone_id
JOIN zones zone
  ON zone.id = workspace.zone_id
 AND zone.status = 'active'
 AND zone.deleted_at IS NULL
WHERE workspace.id = $1::uuid
  AND workspace.status = 'active'
  AND workspace.deleted_at IS NULL
FOR UPDATE OF quota
`, workspaceID).Scan(&enforcementMode, &maximum, &current)
	if errors.Is(err, pgx.ErrNoRows) {
		return webhooksdomain.ErrWebhookQuotaExceeded
	}
	if err != nil {
		return err
	}
	if enforcementMode == "hard" && current >= maximum {
		return webhooksdomain.ErrWebhookQuotaExceeded
	}
	return nil
}

func (r *Repository) ClaimDeliveries(ctx context.Context, limit int) ([]webhooksdomain.Delivery, error) {
	rows, err := r.pool.Query(ctx, `
WITH picked AS (
    SELECT d.id
    FROM webhook_deliveries d
    JOIN outgoing_webhooks ow ON ow.id = d.outgoing_webhook_id AND ow.status = 'active'
    WHERE d.status = 'pending'
       OR (d.status = 'failed' AND (d.next_attempt_at IS NULL OR d.next_attempt_at <= now()))
       OR (d.status = 'delivering' AND d.updated_at < now() - interval '5 minutes')
    ORDER BY d.created_at ASC
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE webhook_deliveries d
SET status = 'delivering'
FROM picked
WHERE d.id = picked.id
RETURNING d.id::text,
          d.outgoing_webhook_id::text,
          (SELECT target_url FROM outgoing_webhooks WHERE id = d.outgoing_webhook_id),
          (SELECT signing_secret_encrypted FROM outgoing_webhooks WHERE id = d.outgoing_webhook_id),
          d.event_id::text,
          d.event_type,
          d.request_body::text,
          d.response_status,
          d.response_body,
          d.status,
          d.attempt_count,
          d.next_attempt_at,
          d.delivered_at,
          d.created_at,
          d.updated_at
`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	deliveries := make([]webhooksdomain.Delivery, 0)
	for rows.Next() {
		delivery, err := scanDelivery(rows)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, delivery)
	}
	return deliveries, rows.Err()
}

func (r *Repository) MarkDeliverySuccess(ctx context.Context, deliveryID string, responseStatus int, responseBody string) error {
	_, err := r.pool.Exec(ctx, `
UPDATE webhook_deliveries
SET status = 'success',
    attempt_count = attempt_count + 1,
    response_status = $2,
    response_body = $3,
    delivered_at = now(),
    next_attempt_at = NULL
WHERE id = $1::uuid
`, deliveryID, responseStatus, trimResponseBody(responseBody))
	return err
}

func (r *Repository) MarkDeliveryFailed(ctx context.Context, deliveryID string, responseStatus int, responseBody string, retryAfter time.Duration, maxRetries int) error {
	_, err := r.pool.Exec(ctx, `
UPDATE webhook_deliveries
SET attempt_count = attempt_count + 1,
    response_status = NULLIF($2, 0),
    response_body = $3,
    status = CASE WHEN attempt_count + 1 >= $5 THEN 'dead' ELSE 'failed' END,
    next_attempt_at = CASE WHEN attempt_count + 1 >= $5 THEN NULL ELSE now() + make_interval(secs => $4::int) END
WHERE id = $1::uuid
`, deliveryID, responseStatus, trimResponseBody(responseBody), int(retryAfter.Seconds()), maxRetries)
	return err
}

type commandExecutor interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func insertIntegrationMessage(ctx context.Context, exec commandExecutor, params webhooksapp.IntegrationMessageParams) (webhooksdomain.IntegrationMessage, error) {
	row := exec.QueryRow(ctx, `
INSERT INTO messages (workspace_id, channel_id, sender_id, kind, body, metadata)
SELECT c.workspace_id, c.id, NULL, $3, $4, $5::jsonb
FROM channels c
WHERE c.workspace_id = $1::uuid
  AND c.id = $2::uuid
  AND c.status = 'active'
  AND c.deleted_at IS NULL
RETURNING id::text, workspace_id::text, channel_id::text, kind, body, metadata::text, created_at
`, params.WorkspaceID, params.ChannelID, params.Kind, params.Body, string(params.Metadata))
	message, err := scanIntegrationMessage(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return webhooksdomain.IntegrationMessage{}, webhooksdomain.ErrIntegrationChannelNotFound
		}
		return webhooksdomain.IntegrationMessage{}, err
	}
	if err := upsertSearchDocument(ctx, exec, message); err != nil {
		return webhooksdomain.IntegrationMessage{}, err
	}
	if err := insertOutbox(ctx, exec, "message", message.ID, "MessageCreated", map[string]any{
		"workspace_id":       message.WorkspaceID,
		"channel_id":         message.ChannelID,
		"message_id":         message.ID,
		"sender_id":          "",
		"mentioned_user_ids": []string{},
	}); err != nil {
		return webhooksdomain.IntegrationMessage{}, err
	}
	return message, nil
}

func upsertSearchDocument(ctx context.Context, exec commandExecutor, message webhooksdomain.IntegrationMessage) error {
	metadata, err := json.Marshal(map[string]any{
		"channel_id": message.ChannelID,
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

func scanIncoming(row rowScanner) (webhooksdomain.IncomingWebhook, error) {
	var webhook webhooksdomain.IncomingWebhook
	var channelID sql.NullString
	var createdBy sql.NullString
	var lastUsedAt sql.NullTime
	if err := row.Scan(
		&webhook.ID,
		&webhook.WorkspaceID,
		&channelID,
		&webhook.Name,
		&webhook.Status,
		&createdBy,
		&webhook.CreatedAt,
		&webhook.UpdatedAt,
		&lastUsedAt,
	); err != nil {
		return webhooksdomain.IncomingWebhook{}, err
	}
	webhook.ChannelID = nullStringPtr(channelID)
	webhook.CreatedBy = nullStringPtr(createdBy)
	webhook.LastUsedAt = nullTimePtr(lastUsedAt)
	return webhook, nil
}

func scanOutgoing(row rowScanner) (webhooksdomain.OutgoingWebhook, error) {
	var webhook webhooksdomain.OutgoingWebhook
	var createdBy sql.NullString
	if err := row.Scan(
		&webhook.ID,
		&webhook.WorkspaceID,
		&webhook.Name,
		&webhook.TargetURL,
		&webhook.SigningSecretEncrypted,
		&webhook.EventTypes,
		&webhook.Status,
		&createdBy,
		&webhook.CreatedAt,
		&webhook.UpdatedAt,
	); err != nil {
		return webhooksdomain.OutgoingWebhook{}, err
	}
	webhook.CreatedBy = nullStringPtr(createdBy)
	return webhook, nil
}

func scanDelivery(row rowScanner) (webhooksdomain.Delivery, error) {
	var delivery webhooksdomain.Delivery
	var eventID sql.NullString
	var responseStatus sql.NullInt64
	var responseBody sql.NullString
	var nextAttemptAt sql.NullTime
	var deliveredAt sql.NullTime
	var requestBody string
	if err := row.Scan(
		&delivery.ID,
		&delivery.OutgoingWebhookID,
		&delivery.TargetURL,
		&delivery.SigningSecretEncrypted,
		&eventID,
		&delivery.EventType,
		&requestBody,
		&responseStatus,
		&responseBody,
		&delivery.Status,
		&delivery.AttemptCount,
		&nextAttemptAt,
		&deliveredAt,
		&delivery.CreatedAt,
		&delivery.UpdatedAt,
	); err != nil {
		return webhooksdomain.Delivery{}, err
	}
	delivery.EventID = nullStringPtr(eventID)
	if responseStatus.Valid {
		value := int(responseStatus.Int64)
		delivery.ResponseStatus = &value
	}
	delivery.ResponseBody = nullStringPtr(responseBody)
	delivery.NextAttemptAt = nullTimePtr(nextAttemptAt)
	delivery.DeliveredAt = nullTimePtr(deliveredAt)
	delivery.RequestBody = []byte(requestBody)
	return delivery, nil
}

func scanIntegrationMessage(row rowScanner) (webhooksdomain.IntegrationMessage, error) {
	var message webhooksdomain.IntegrationMessage
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
		return webhooksdomain.IntegrationMessage{}, err
	}
	message.Metadata = []byte(metadata)
	return message, nil
}

func nullableString(value *string) any {
	if value == nil {
		return nil
	}
	return *value
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

func trimResponseBody(value string) string {
	if len(value) > 4000 {
		return value[:4000]
	}
	return value
}
