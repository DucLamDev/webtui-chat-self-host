package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	messagesapp "github.com/duclamdev/application-chat/backend/internal/modules/messages/application"
	messagesdomain "github.com/duclamdev/application-chat/backend/internal/modules/messages/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) ScheduleMessage(ctx context.Context, params messagesapp.ScheduleMessageParams) (messagesapp.ScheduledMessageDTO, error) {
	mentioned, err := json.Marshal(params.MentionedUserIDs)
	if err != nil {
		return messagesapp.ScheduledMessageDTO{}, err
	}
	row := r.pool.QueryRow(ctx, `
INSERT INTO scheduled_messages (
    workspace_id, channel_id, sender_id, parent_id, kind, body, metadata,
    mentioned_user_ids, client_message_id, scheduled_for
)
SELECT c.workspace_id, c.id, $3::uuid, NULLIF($4, '')::uuid, $5, $6,
       $7::jsonb, $8::jsonb, NULLIF($9, ''), $10
FROM channels c
JOIN channel_members cm
  ON cm.channel_id = c.id
 AND cm.user_id = $3::uuid
 AND cm.status IN ('active', 'muted')
WHERE c.workspace_id = $1::uuid
  AND c.id = $2::uuid
  AND c.status = 'active'
  AND c.deleted_at IS NULL
  AND (
      $4 = ''
      OR EXISTS (
          SELECT 1 FROM messages m
          WHERE m.workspace_id = c.workspace_id
            AND m.channel_id = c.id
            AND m.id = $4::uuid
            AND m.deleted_at IS NULL
      )
  )
RETURNING id::text, workspace_id::text, channel_id::text, sender_id::text,
          parent_id::text, kind, body, metadata::text, scheduled_for, status,
          sent_message_id::text, attempt_count, last_error, client_message_id,
          created_at, updated_at
`, params.WorkspaceID, params.ChannelID, params.SenderID, params.ParentID,
		params.Kind, params.Body, string(params.Metadata), string(mentioned),
		params.ClientMessageID, params.ScheduledFor)
	dto, err := scanScheduledMessage(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return messagesapp.ScheduledMessageDTO{}, messagesdomain.ErrChannelNotFound
	}
	return dto, err
}

func (r *Repository) ListScheduledMessages(ctx context.Context, params messagesapp.UserProductivityParams) ([]messagesapp.ScheduledMessageDTO, error) {
	rows, err := r.pool.Query(ctx, `
SELECT sm.id::text, sm.workspace_id::text, sm.channel_id::text, sm.sender_id::text,
       sm.parent_id::text, sm.kind, sm.body, sm.metadata::text, sm.scheduled_for,
       sm.status, sm.sent_message_id::text, sm.attempt_count, sm.last_error,
       sm.client_message_id, sm.created_at, sm.updated_at
FROM scheduled_messages sm
JOIN channel_members cm
  ON cm.channel_id = sm.channel_id
 AND cm.user_id = $2::uuid
 AND cm.status IN ('active', 'muted')
WHERE sm.workspace_id = $1::uuid
  AND sm.sender_id = $2::uuid
  AND ($3 = '' OR sm.channel_id = $3::uuid)
ORDER BY
  CASE WHEN sm.status = 'pending' THEN 0 ELSE 1 END,
  sm.scheduled_for ASC
LIMIT $4
`, params.WorkspaceID, params.UserID, params.ChannelID, params.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]messagesapp.ScheduledMessageDTO, 0)
	for rows.Next() {
		dto, err := scanScheduledMessage(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, dto)
	}
	return result, rows.Err()
}

func (r *Repository) CancelScheduledMessage(ctx context.Context, params messagesapp.ScheduledMessageRef) error {
	command, err := r.pool.Exec(ctx, `
UPDATE scheduled_messages
SET status = 'cancelled', last_error = NULL
WHERE workspace_id = $1::uuid
  AND sender_id = $2::uuid
  AND id = $3::uuid
  AND status IN ('pending', 'failed')
`, params.WorkspaceID, params.SenderID, params.ID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return messagesdomain.ErrMessageNotFound
	}
	return nil
}

type scheduledJob struct {
	id               string
	workspaceID      string
	channelID        string
	senderID         string
	parentID         string
	kind             string
	body             string
	metadata         []byte
	mentionedUserIDs []string
	clientMessageID  string
}

func (r *Repository) ProcessDueScheduledMessages(
	ctx context.Context,
	limit int,
	authorizer messagesapp.ScheduledMessageDeliveryAuthorizer,
) (int, error) {
	if authorizer == nil {
		return 0, errors.New("scheduled message delivery authorizer is required")
	}
	if r.scheduledTermsVersion == "" || r.scheduledPrivacyVersion == "" {
		return 0, errors.New("scheduled message legal document versions are required")
	}
	rows, err := r.pool.Query(ctx, `
WITH picked AS (
    SELECT id
    FROM scheduled_messages
    WHERE scheduled_for <= now()
      AND (
          status = 'pending'
          OR (
              status = 'failed'
              AND attempt_count < 5
              AND updated_at <= now() - make_interval(secs => LEAST(300, GREATEST(15, attempt_count * 30)))
          )
          OR (status = 'processing' AND updated_at < now() - interval '5 minutes')
      )
    ORDER BY scheduled_for, created_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
),
claimed AS (
    UPDATE scheduled_messages sm
    SET status = 'processing',
        attempt_count = attempt_count + 1,
        last_error = NULL
    FROM picked
    WHERE sm.id = picked.id
    RETURNING sm.id::text, sm.workspace_id::text, sm.channel_id::text,
              sm.sender_id::text, sm.parent_id::text, sm.kind, sm.body,
              sm.metadata::text, sm.mentioned_user_ids::text,
              COALESCE(sm.client_message_id, '')
)
SELECT * FROM claimed
`, limit)
	if err != nil {
		return 0, err
	}
	jobs := make([]scheduledJob, 0)
	for rows.Next() {
		var job scheduledJob
		var parentID sql.NullString
		var metadata string
		var mentioned string
		if err := rows.Scan(
			&job.id, &job.workspaceID, &job.channelID, &job.senderID, &parentID,
			&job.kind, &job.body, &metadata, &mentioned, &job.clientMessageID,
		); err != nil {
			rows.Close()
			return 0, err
		}
		if parentID.Valid {
			job.parentID = parentID.String
		}
		job.metadata = []byte(metadata)
		if err := json.Unmarshal([]byte(mentioned), &job.mentionedUserIDs); err != nil {
			rows.Close()
			return 0, err
		}
		if job.clientMessageID == "" {
			job.clientMessageID = "scheduled:" + job.id
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	processed := 0
	var processingErrors []error
	for _, job := range jobs {
		authorizationErr := authorizer.AuthorizeScheduledMessageDelivery(ctx, messagesapp.ScheduledMessageDelivery{
			WorkspaceID: job.workspaceID,
			ChannelID:   job.channelID,
			SenderID:    job.senderID,
		})
		if authorizationErr != nil {
			if reason, terminal := scheduledDeliveryCancellationReason(authorizationErr); terminal {
				if updateErr := setScheduledMessageDeliveryState(ctx, r.pool, job.id, "cancelled", reason, ""); updateErr != nil {
					processingErrors = append(processingErrors, fmt.Errorf("cancel scheduled message %s: %w", job.id, updateErr))
				}
				continue
			}
			if updateErr := setScheduledMessageDeliveryState(ctx, r.pool, job.id, "failed", authorizationErr.Error(), ""); updateErr != nil {
				processingErrors = append(processingErrors, fmt.Errorf("record scheduled message %s authorization failure: %w", job.id, updateErr))
			}
			processingErrors = append(processingErrors, fmt.Errorf("authorize scheduled message %s: %w", job.id, authorizationErr))
			continue
		}
		message, sendErr := r.Send(ctx, messagesapp.SendParams{
			WorkspaceID:                    job.workspaceID,
			ChannelID:                      job.channelID,
			SenderID:                       job.senderID,
			ParentID:                       job.parentID,
			ClientMessageID:                job.clientMessageID,
			Kind:                           job.kind,
			Body:                           job.body,
			Metadata:                       ensureScheduledClientID(job.metadata, job.clientMessageID, job.id),
			MentionedUserIDs:               job.mentionedUserIDs,
			EnforceScheduledDeliveryPolicy: true,
			ScheduledTermsVersion:          r.scheduledTermsVersion,
			ScheduledPrivacyVersion:        r.scheduledPrivacyVersion,
		})
		if sendErr != nil {
			if reason, terminal := scheduledDeliveryCancellationReason(sendErr); terminal {
				if updateErr := setScheduledMessageDeliveryState(ctx, r.pool, job.id, "cancelled", reason, ""); updateErr != nil {
					processingErrors = append(processingErrors, fmt.Errorf("cancel scheduled message %s: %w", job.id, updateErr))
				}
				continue
			}
			if updateErr := setScheduledMessageDeliveryState(ctx, r.pool, job.id, "failed", sendErr.Error(), ""); updateErr != nil {
				processingErrors = append(processingErrors, fmt.Errorf("record scheduled message %s send failure: %w", job.id, updateErr))
			}
			processingErrors = append(processingErrors, fmt.Errorf("scheduled message %s: %w", job.id, sendErr))
			continue
		}
		if err := setScheduledMessageDeliveryState(ctx, r.pool, job.id, "sent", "", message.ID); err != nil {
			processingErrors = append(processingErrors, fmt.Errorf("mark scheduled message %s sent: %w", job.id, err))
			continue
		}
		processed++
	}
	return processed, errors.Join(processingErrors...)
}

func setScheduledMessageDeliveryState(
	ctx context.Context,
	exec commandExecutor,
	jobID string,
	status string,
	lastError string,
	sentMessageID string,
) error {
	command, err := exec.Exec(ctx, `
UPDATE scheduled_messages
SET status = $2,
    last_error = left(NULLIF($3, ''), 1000),
    sent_message_id = CASE WHEN $2 = 'sent' THEN NULLIF($4, '')::uuid ELSE sent_message_id END
WHERE id = $1::uuid
`, jobID, status, lastError, sentMessageID)
	if err != nil {
		return err
	}
	if command.RowsAffected() != 1 {
		return fmt.Errorf("scheduled message state update affected %d rows", command.RowsAffected())
	}
	return nil
}

func scheduledDeliveryCancellationReason(err error) (string, bool) {
	switch {
	case errors.Is(err, messagesdomain.ErrScheduledDeliveryPermissionRevoked):
		return "sender permission revoked before delivery", true
	case errors.Is(err, messagesdomain.ErrScheduledDeliveryLegalAcceptanceStale):
		return "current legal documents not accepted before delivery", true
	case errors.Is(err, messagesdomain.ErrInteractionBlocked):
		return "interaction blocked before delivery", true
	default:
		return "", false
	}
}

func ensureScheduledClientID(metadata []byte, clientMessageID string, scheduledID string) []byte {
	payload := map[string]any{}
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &payload)
	}
	if payload == nil {
		payload = map[string]any{}
	}
	payload["client_message_id"] = clientMessageID
	payload["scheduled_message_id"] = scheduledID
	encoded, err := json.Marshal(payload)
	if err != nil {
		return metadata
	}
	return encoded
}

func (r *Repository) CreateReminder(ctx context.Context, params messagesapp.CreateReminderParams) (messagesapp.MessageReminderDTO, error) {
	row := r.pool.QueryRow(ctx, `
INSERT INTO message_reminders (
    workspace_id, channel_id, message_id, user_id, remind_at, note
)
SELECT m.workspace_id, m.channel_id, m.id, $4::uuid, $5, NULLIF($6, '')
FROM messages m
JOIN channel_members cm
  ON cm.channel_id = m.channel_id
 AND cm.user_id = $4::uuid
 AND cm.status IN ('active', 'muted')
WHERE m.workspace_id = $1::uuid
  AND m.channel_id = $2::uuid
  AND m.id = $3::uuid
  AND m.deleted_at IS NULL
ON CONFLICT (workspace_id, message_id, user_id)
    WHERE status IN ('pending', 'processing', 'failed')
DO UPDATE SET
    remind_at = EXCLUDED.remind_at,
    note = EXCLUDED.note,
    status = 'pending',
    notification_id = NULL,
    attempt_count = 0,
    last_error = NULL
RETURNING id::text, workspace_id::text, channel_id::text, message_id::text,
          user_id::text, remind_at, note, status, notification_id::text,
          created_at, updated_at
`, params.WorkspaceID, params.ChannelID, params.MessageID, params.UserID, params.RemindAt, params.Note)
	dto, err := scanReminder(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return messagesapp.MessageReminderDTO{}, messagesdomain.ErrMessageNotFound
	}
	return dto, err
}

func (r *Repository) ListReminders(ctx context.Context, params messagesapp.UserProductivityParams) ([]messagesapp.MessageReminderDTO, error) {
	rows, err := r.pool.Query(ctx, `
SELECT mr.id::text, mr.workspace_id::text, mr.channel_id::text,
       mr.message_id::text, mr.user_id::text, mr.remind_at, mr.note,
       mr.status, mr.notification_id::text, mr.created_at, mr.updated_at
FROM message_reminders mr
JOIN channel_members cm
  ON cm.channel_id = mr.channel_id
 AND cm.user_id = $2::uuid
 AND cm.status IN ('active', 'muted')
WHERE mr.workspace_id = $1::uuid
  AND mr.user_id = $2::uuid
  AND ($3 = '' OR mr.channel_id = $3::uuid)
ORDER BY
  CASE WHEN mr.status = 'pending' THEN 0 ELSE 1 END,
  mr.remind_at ASC
LIMIT $4
`, params.WorkspaceID, params.UserID, params.ChannelID, params.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]messagesapp.MessageReminderDTO, 0)
	for rows.Next() {
		dto, err := scanReminder(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, dto)
	}
	return result, rows.Err()
}

func (r *Repository) CancelReminder(ctx context.Context, params messagesapp.ReminderRef) error {
	command, err := r.pool.Exec(ctx, `
UPDATE message_reminders
SET status = 'cancelled', last_error = NULL
WHERE workspace_id = $1::uuid
  AND user_id = $2::uuid
  AND id = $3::uuid
  AND status IN ('pending', 'failed')
`, params.WorkspaceID, params.UserID, params.ID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return messagesdomain.ErrMessageNotFound
	}
	return nil
}

func (r *Repository) ProcessDueReminders(ctx context.Context, limit int) (int, error) {
	rows, err := r.pool.Query(ctx, `
WITH picked AS (
    SELECT id
    FROM message_reminders
    WHERE remind_at <= now()
      AND (
          status = 'pending'
          OR (status = 'failed' AND attempt_count < 5 AND updated_at < now() - interval '30 seconds')
          OR (status = 'processing' AND updated_at < now() - interval '5 minutes')
      )
    ORDER BY remind_at, created_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
),
claimed AS (
    UPDATE message_reminders mr
    SET status = 'processing',
        attempt_count = attempt_count + 1,
        last_error = NULL
    FROM picked
    WHERE mr.id = picked.id
    RETURNING mr.id::text, mr.workspace_id::text, mr.channel_id::text,
              mr.message_id::text, mr.user_id::text, COALESCE(mr.note, '')
)
SELECT * FROM claimed
`, limit)
	if err != nil {
		return 0, err
	}
	type reminderJob struct {
		id, workspaceID, channelID, messageID, userID, note string
	}
	jobs := make([]reminderJob, 0)
	for rows.Next() {
		var job reminderJob
		if err := rows.Scan(&job.id, &job.workspaceID, &job.channelID, &job.messageID, &job.userID, &job.note); err != nil {
			rows.Close()
			return 0, err
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	processed := 0
	var processingErrors []error
	for _, job := range jobs {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return processed, err
		}
		var notificationID string
		var payload string
		err = tx.QueryRow(ctx, `
WITH source AS (
    SELECT m.body, COALESCE(NULLIF($6, ''), left(m.body, 160), 'Tin nhắn đã lưu') AS reminder_body
    FROM messages m
    WHERE m.workspace_id = $2::uuid
      AND m.channel_id = $3::uuid
      AND m.id = $4::uuid
), inserted AS (
    INSERT INTO notifications (
        user_id, workspace_id, channel_id, message_id, type, title, body, data
    )
    SELECT $5::uuid, $2::uuid, $3::uuid, $4::uuid, 'reminder',
           'Nhắc lại tin nhắn', source.reminder_body,
           jsonb_build_object(
               'event_id', 'reminder:' || $1,
               'event_type', 'message_reminder',
               'target_type', 'message',
               'workspace_id', $2,
               'channel_id', $3,
               'message_id', $4,
               'title', 'Nhắc lại tin nhắn',
               'body', source.reminder_body,
               'deep_link', 'webtui://chat/conversations/' || $3 || '?workspaceId=' || $2 || '&messageId=' || $4
           )
    FROM source
    WHERE NOT EXISTS (
        SELECT 1
        FROM notifications existing
        WHERE existing.user_id = $5::uuid
          AND existing.data->>'event_id' = 'reminder:' || $1
    )
    RETURNING id::text, data::text
)
SELECT id, data FROM inserted
UNION ALL
SELECT n.id::text, n.data::text
FROM notifications n
WHERE n.user_id = $5::uuid
  AND n.data->>'event_id' = 'reminder:' || $1
LIMIT 1
`, job.id, job.workspaceID, job.channelID, job.messageID, job.userID, job.note).Scan(&notificationID, &payload)
		if err == nil {
			_, err = tx.Exec(ctx, `
INSERT INTO notification_jobs (notification_id, workspace_id, user_id, channel, payload)
SELECT $1::uuid, $2::uuid, $3::uuid, 'push', $4::jsonb
WHERE NOT EXISTS (
    SELECT 1 FROM notification_jobs
    WHERE notification_id = $1::uuid AND channel = 'push'
)
`, notificationID, job.workspaceID, job.userID, payload)
		}
		if err == nil {
			_, err = tx.Exec(ctx, `
UPDATE message_reminders
SET status = 'fired', notification_id = $2::uuid, last_error = NULL
WHERE id = $1::uuid
`, job.id, notificationID)
		}
		if err == nil {
			err = tx.Commit(ctx)
		} else {
			_ = tx.Rollback(ctx)
		}
		if err != nil {
			_, _ = r.pool.Exec(ctx, `
UPDATE message_reminders
SET status = 'failed', last_error = left($2, 1000)
WHERE id = $1::uuid
`, job.id, err.Error())
			processingErrors = append(processingErrors, fmt.Errorf("reminder %s: %w", job.id, err))
			continue
		}
		processed++
	}
	return processed, errors.Join(processingErrors...)
}

func (r *Repository) GetThreadDetails(ctx context.Context, params messagesapp.ThreadDetailsParams) (messagesapp.ThreadDetailsDTO, error) {
	return scanThreadDetails(r.pool.QueryRow(ctx, threadDetailsSelect+`
WHERE mt.workspace_id = $1::uuid
  AND mt.channel_id = $2::uuid
  AND mt.root_message_id = $3::uuid
  AND cm.user_id = $4::uuid
`, params.WorkspaceID, params.ChannelID, params.RootID, params.UserID))
}

func (r *Repository) UpsertThreadDetails(ctx context.Context, params messagesapp.UpsertThreadDetailsParams) (messagesapp.ThreadDetailsDTO, error) {
	command, err := r.pool.Exec(ctx, `
INSERT INTO message_threads (
    workspace_id, channel_id, root_message_id, title, description, status,
    created_by, resolved_by, resolved_at
)
SELECT m.workspace_id, m.channel_id, m.id, $5, $6, $7, $4::uuid,
       CASE WHEN $7 = 'resolved' THEN $4::uuid ELSE NULL END,
       CASE WHEN $7 = 'resolved' THEN now() ELSE NULL END
FROM messages m
JOIN channel_members cm
  ON cm.channel_id = m.channel_id
 AND cm.user_id = $4::uuid
 AND cm.status IN ('active', 'muted')
WHERE m.workspace_id = $1::uuid
  AND m.channel_id = $2::uuid
  AND m.id = $3::uuid
  AND COALESCE(m.thread_root_id, m.id) = m.id
  AND m.deleted_at IS NULL
ON CONFLICT (workspace_id, root_message_id)
DO UPDATE SET
    title = EXCLUDED.title,
    description = EXCLUDED.description,
    status = EXCLUDED.status,
    resolved_by = EXCLUDED.resolved_by,
    resolved_at = EXCLUDED.resolved_at
`, params.WorkspaceID, params.ChannelID, params.RootID, params.UserID, params.Title, params.Description, params.Status)
	if err != nil {
		return messagesapp.ThreadDetailsDTO{}, err
	}
	if command.RowsAffected() == 0 {
		return messagesapp.ThreadDetailsDTO{}, messagesdomain.ErrMessageNotFound
	}
	_, _ = r.pool.Exec(ctx, `
INSERT INTO message_thread_subscriptions (workspace_id, root_message_id, user_id, subscribed)
VALUES ($1::uuid, $2::uuid, $3::uuid, true)
ON CONFLICT (workspace_id, root_message_id, user_id)
DO UPDATE SET subscribed = true
`, params.WorkspaceID, params.RootID, params.UserID)
	return r.GetThreadDetails(ctx, params.ThreadDetailsParams)
}

func (r *Repository) ListThreadDetails(ctx context.Context, params messagesapp.ListThreadDetailsParams) ([]messagesapp.ThreadDetailsDTO, error) {
	rows, err := r.pool.Query(ctx, threadDetailsSelect+`
WHERE mt.workspace_id = $1::uuid
  AND ($2 = '' OR mt.channel_id = $2::uuid)
  AND cm.user_id = $3::uuid
  AND (NOT $4 OR COALESCE(mts.subscribed, false))
ORDER BY COALESCE(last_reply.last_reply_at, mt.updated_at) DESC
LIMIT $5
`, params.WorkspaceID, params.ChannelID, params.UserID, params.SubscribedOnly, params.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]messagesapp.ThreadDetailsDTO, 0)
	for rows.Next() {
		dto, err := scanThreadDetails(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, dto)
	}
	return result, rows.Err()
}

func (r *Repository) SetThreadSubscription(ctx context.Context, params messagesapp.ThreadSubscriptionParams) (messagesapp.ThreadDetailsDTO, error) {
	command, err := r.pool.Exec(ctx, `
INSERT INTO message_thread_subscriptions (
    workspace_id, root_message_id, user_id, subscribed
)
SELECT mt.workspace_id, mt.root_message_id, $4::uuid, $5
FROM message_threads mt
JOIN channel_members cm
  ON cm.channel_id = mt.channel_id
 AND cm.user_id = $4::uuid
 AND cm.status IN ('active', 'muted')
WHERE mt.workspace_id = $1::uuid
  AND mt.channel_id = $2::uuid
  AND mt.root_message_id = $3::uuid
ON CONFLICT (workspace_id, root_message_id, user_id)
DO UPDATE SET subscribed = EXCLUDED.subscribed
`, params.WorkspaceID, params.ChannelID, params.RootID, params.UserID, params.Subscribed)
	if err != nil {
		return messagesapp.ThreadDetailsDTO{}, err
	}
	if command.RowsAffected() == 0 {
		return messagesapp.ThreadDetailsDTO{}, messagesdomain.ErrMessageNotFound
	}
	return r.GetThreadDetails(ctx, params.ThreadDetailsParams)
}

func (r *Repository) MarkThreadRead(ctx context.Context, params messagesapp.ThreadReadParams) (messagesapp.ThreadDetailsDTO, error) {
	command, err := r.pool.Exec(ctx, `
INSERT INTO message_thread_subscriptions (
    workspace_id, root_message_id, user_id, subscribed, last_read_at,
    last_read_message_id
)
SELECT mt.workspace_id, mt.root_message_id, $4::uuid, true, now(),
       CASE WHEN $5 = '' THEN NULL ELSE $5::uuid END
FROM message_threads mt
JOIN channel_members cm
  ON cm.channel_id = mt.channel_id
 AND cm.user_id = $4::uuid
 AND cm.status IN ('active', 'muted')
WHERE mt.workspace_id = $1::uuid
  AND mt.channel_id = $2::uuid
  AND mt.root_message_id = $3::uuid
  AND (
      $5 = ''
      OR EXISTS (
          SELECT 1 FROM messages m
          WHERE m.workspace_id = mt.workspace_id
            AND m.channel_id = mt.channel_id
            AND m.id = $5::uuid
            AND (m.id = mt.root_message_id OR m.thread_root_id = mt.root_message_id)
      )
  )
ON CONFLICT (workspace_id, root_message_id, user_id)
DO UPDATE SET
    last_read_at = now(),
    last_read_message_id = EXCLUDED.last_read_message_id
`, params.WorkspaceID, params.ChannelID, params.RootID, params.UserID, params.LastReadMessageID)
	if err != nil {
		return messagesapp.ThreadDetailsDTO{}, err
	}
	if command.RowsAffected() == 0 {
		return messagesapp.ThreadDetailsDTO{}, messagesdomain.ErrMessageNotFound
	}
	return r.GetThreadDetails(ctx, params.ThreadDetailsParams)
}

const threadDetailsSelect = `
SELECT mt.workspace_id::text, mt.channel_id::text, mt.root_message_id::text,
       mt.title, mt.description, mt.status,
       COALESCE(mts.subscribed, false),
       COALESCE(reply_stats.reply_count, 0)::int,
       COALESCE((
           SELECT count(*)::int
           FROM messages unread_message
           WHERE unread_message.workspace_id = mt.workspace_id
             AND unread_message.channel_id = mt.channel_id
             AND unread_message.thread_root_id = mt.root_message_id
             AND unread_message.deleted_at IS NULL
             AND unread_message.sender_id <> cm.user_id
             AND unread_message.created_at > COALESCE(mts.last_read_at, 'epoch'::timestamptz)
       ), 0)::int AS unread_count,
       last_reply.last_reply_at,
       mts.last_read_at,
       mt.created_at,
       mt.updated_at
FROM message_threads mt
JOIN channel_members cm
  ON cm.channel_id = mt.channel_id
 AND cm.status IN ('active', 'muted')
LEFT JOIN message_thread_subscriptions mts
  ON mts.workspace_id = mt.workspace_id
 AND mts.root_message_id = mt.root_message_id
 AND mts.user_id = cm.user_id
LEFT JOIN LATERAL (
    SELECT count(*)::int AS reply_count
    FROM messages reply
    WHERE reply.workspace_id = mt.workspace_id
      AND reply.channel_id = mt.channel_id
      AND reply.thread_root_id = mt.root_message_id
      AND reply.deleted_at IS NULL
) reply_stats ON true
LEFT JOIN LATERAL (
    SELECT max(reply.created_at) AS last_reply_at
    FROM messages reply
    WHERE reply.workspace_id = mt.workspace_id
      AND reply.channel_id = mt.channel_id
      AND reply.thread_root_id = mt.root_message_id
      AND reply.deleted_at IS NULL
) last_reply ON true
`

func scanScheduledMessage(row rowScanner) (messagesapp.ScheduledMessageDTO, error) {
	var dto messagesapp.ScheduledMessageDTO
	var parentID, sentMessageID, lastError, clientMessageID sql.NullString
	var metadata string
	var scheduledFor, createdAt, updatedAt time.Time
	if err := row.Scan(
		&dto.ID, &dto.WorkspaceID, &dto.ChannelID, &dto.SenderID, &parentID,
		&dto.Kind, &dto.Body, &metadata, &scheduledFor, &dto.Status,
		&sentMessageID, &dto.AttemptCount, &lastError, &clientMessageID,
		&createdAt, &updatedAt,
	); err != nil {
		return messagesapp.ScheduledMessageDTO{}, err
	}
	dto.ParentID = nullStringPtr(parentID)
	dto.SentMessageID = nullStringPtr(sentMessageID)
	dto.LastError = nullStringPtr(lastError)
	dto.ClientMessageID = nullStringPtr(clientMessageID)
	dto.Metadata = json.RawMessage(metadata)
	dto.ScheduledFor = scheduledFor.UTC().Format(time.RFC3339Nano)
	dto.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	dto.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	return dto, nil
}

func scanReminder(row rowScanner) (messagesapp.MessageReminderDTO, error) {
	var dto messagesapp.MessageReminderDTO
	var note, notificationID sql.NullString
	var remindAt, createdAt, updatedAt time.Time
	if err := row.Scan(
		&dto.ID, &dto.WorkspaceID, &dto.ChannelID, &dto.MessageID, &dto.UserID,
		&remindAt, &note, &dto.Status, &notificationID, &createdAt, &updatedAt,
	); err != nil {
		return messagesapp.MessageReminderDTO{}, err
	}
	dto.Note = nullStringPtr(note)
	dto.NotificationID = nullStringPtr(notificationID)
	dto.RemindAt = remindAt.UTC().Format(time.RFC3339Nano)
	dto.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	dto.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	return dto, nil
}

func scanThreadDetails(row rowScanner) (messagesapp.ThreadDetailsDTO, error) {
	var dto messagesapp.ThreadDetailsDTO
	var lastReplyAt, lastReadAt sql.NullTime
	var createdAt, updatedAt time.Time
	if err := row.Scan(
		&dto.WorkspaceID, &dto.ChannelID, &dto.RootMessageID,
		&dto.Title, &dto.Description, &dto.Status, &dto.Subscribed,
		&dto.ReplyCount, &dto.UnreadCount, &lastReplyAt, &lastReadAt,
		&createdAt, &updatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return messagesapp.ThreadDetailsDTO{}, messagesdomain.ErrMessageNotFound
		}
		return messagesapp.ThreadDetailsDTO{}, err
	}
	dto.LastReplyAt = formatNullTime(lastReplyAt)
	dto.LastReadAt = formatNullTime(lastReadAt)
	dto.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	dto.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	return dto, nil
}

func formatNullTime(value sql.NullTime) *string {
	if !value.Valid {
		return nil
	}
	formatted := value.Time.UTC().Format(time.RFC3339Nano)
	return &formatted
}

func normalizeNullableString(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return strings.TrimSpace(value.String)
}
