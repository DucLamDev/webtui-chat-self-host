package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	notificationsapp "github.com/duclamdev/application-chat/backend/internal/modules/notifications/application"
	notificationsdomain "github.com/duclamdev/application-chat/backend/internal/modules/notifications/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool       *pgxpool.Pool
	pushSender PushSender
}

type PushSender interface {
	Enabled() bool
	Send(ctx context.Context, token string, payload map[string]any) error
}

func NewRepository(pool *pgxpool.Pool, pushSenders ...PushSender) *Repository {
	repository := &Repository{pool: pool}
	if len(pushSenders) > 0 {
		repository.pushSender = pushSenders[0]
	}
	return repository
}

func (r *Repository) CreateMentionNotifications(ctx context.Context, params notificationsapp.MentionParams) error {
	for _, userID := range uniqueMentions(params.MentionedUserIDs, params.SenderID) {
		data, err := json.Marshal(map[string]any{
			"event_id":     params.EventID,
			"workspace_id": params.WorkspaceID,
			"channel_id":   params.ChannelID,
			"message_id":   params.MessageID,
			"sender_id":    params.SenderID,
		})
		if err != nil {
			return err
		}
		var notificationID string
		err = r.pool.QueryRow(ctx, `
INSERT INTO notifications (user_id, workspace_id, channel_id, message_id, type, title, body, data)
SELECT $1::uuid, $2::uuid, $3::uuid, $4::uuid, 'mention', 'Bạn được nhắc trong một tin nhắn', '', $5::jsonb
WHERE NOT EXISTS (
    SELECT 1
    FROM notifications
    WHERE user_id = $1::uuid AND data->>'event_id' = $6
)
RETURNING id::text
`, userID, params.WorkspaceID, params.ChannelID, params.MessageID, string(data), params.EventID).Scan(&notificationID)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return err
		}
		if err := r.createJob(ctx, notificationID, params.WorkspaceID, userID, data); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) CreateMessageNotifications(ctx context.Context, params notificationsapp.MessageNotificationParams) error {
	rows, err := r.pool.Query(ctx, `
WITH source AS (
    SELECT m.id, m.workspace_id, m.channel_id, m.sender_id, m.kind, m.body,
           COALESCE(NULLIF(trim(u.display_name), ''), u.username::text, 'WebTui') AS sender_name
    FROM messages m
    LEFT JOIN users u ON u.id = m.sender_id
    WHERE m.workspace_id = $2::uuid
      AND m.channel_id = $3::uuid
      AND m.id = $4::uuid
      AND NOT (m.kind = 'event' AND m.metadata ->> 'message_type' = 'call')
), recipients AS (
    SELECT cm.user_id,
           s.*,
           cm.user_id = ANY($6::uuid[]) AS mentioned,
           COALESCE(np.preview, true) AS preview
    FROM source s
    JOIN channel_members cm ON cm.channel_id = s.channel_id
    LEFT JOIN notification_preferences np
      ON np.user_id = cm.user_id AND np.workspace_id = s.workspace_id
    LEFT JOIN notification_channel_preferences ncp
      ON ncp.user_id = cm.user_id
     AND ncp.workspace_id = s.workspace_id
     AND ncp.channel_id = s.channel_id
    WHERE cm.user_id <> $5::uuid
      AND cm.status IN ('active', 'muted')
      AND COALESCE(ncp.mode, 'all') <> 'muted'
      AND (ncp.muted_until IS NULL OR ncp.muted_until <= now())
      AND (
          COALESCE(np.mode, 'all') = 'all'
          OR (COALESCE(np.mode, 'all') = 'mentions' AND cm.user_id = ANY($6::uuid[]))
      )
), inserted AS (
    INSERT INTO notifications (user_id, workspace_id, channel_id, message_id, type, title, body, data)
    SELECT r.user_id,
           r.workspace_id,
           r.channel_id,
           r.id,
           CASE WHEN r.mentioned THEN 'mention' ELSE 'message' END,
           CASE WHEN r.mentioned THEN r.sender_name || ' đã nhắc đến bạn' ELSE r.sender_name END,
           CASE
             WHEN NOT r.preview THEN 'Bạn có tin nhắn mới'
             WHEN length(trim(r.body)) > 0 THEN left(r.body, 180)
             WHEN r.kind = 'event' THEN 'Hoạt động mới trong cuộc trò chuyện'
             ELSE 'Đã gửi một tệp đính kèm'
           END,
           jsonb_build_object(
             'event_id', $1,
             'event_type', 'message',
             'target_type', 'message',
             'workspace_id', r.workspace_id::text,
             'channel_id', r.channel_id::text,
             'message_id', r.id::text,
             'sender_id', r.sender_id::text,
             'title', CASE WHEN r.mentioned THEN r.sender_name || ' đã nhắc đến bạn' ELSE r.sender_name END,
             'body', CASE
               WHEN NOT r.preview THEN 'Bạn có tin nhắn mới'
               WHEN length(trim(r.body)) > 0 THEN left(r.body, 180)
               WHEN r.kind = 'event' THEN 'Hoạt động mới trong cuộc trò chuyện'
               ELSE 'Đã gửi một tệp đính kèm'
             END,
             'deep_link', 'webtui://chat/conversations/' || r.channel_id::text || '?workspaceId=' || r.workspace_id::text
           )
    FROM recipients r
    WHERE NOT EXISTS (
        SELECT 1 FROM notifications n
        WHERE n.user_id = r.user_id AND n.data->>'event_id' = $1
    )
    RETURNING id::text, workspace_id::text, user_id::text, data::text
)
SELECT id, workspace_id, user_id, data FROM inserted
`, params.EventID, params.WorkspaceID, params.ChannelID, params.MessageID, params.SenderID, params.MentionedUserIDs)
	if err != nil {
		return err
	}
	defer rows.Close()

	type pendingJob struct {
		notificationID string
		workspaceID    string
		userID         string
		payload        []byte
	}
	jobs := make([]pendingJob, 0)
	for rows.Next() {
		var job pendingJob
		var payload string
		if err := rows.Scan(&job.notificationID, &job.workspaceID, &job.userID, &payload); err != nil {
			return err
		}
		job.payload = []byte(payload)
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	for _, job := range jobs {
		if err := r.createJob(ctx, job.notificationID, job.workspaceID, job.userID, job.payload); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) CreateIncomingCallNotification(ctx context.Context, params notificationsapp.CallNotificationParams) error {
	data, err := json.Marshal(map[string]any{
		"event_id":          "call:" + params.CallID + ":invite",
		"event_type":        "call_invite",
		"target_type":       "call",
		"call_id":           params.CallID,
		"workspace_id":      params.WorkspaceID,
		"channel_id":        params.ChannelID,
		"initiator_user_id": params.InitiatorUserID,
		"target_user_id":    params.TargetUserID,
		"mode":              params.Mode,
		"status":            "ringing",
		"tag":               "call-" + params.CallID,
		"deep_link":         "webtui://chat/conversations/" + params.ChannelID + "?workspaceId=" + params.WorkspaceID,
	})
	if err != nil {
		return err
	}
	var notificationID string
	var title string
	var body string
	err = r.pool.QueryRow(ctx, `
WITH caller AS (
    SELECT COALESCE(NULLIF(trim(display_name), ''), username::text, 'WebTui') AS display_name
    FROM users
    WHERE id = $4::uuid
), inserted AS (
    INSERT INTO notifications (user_id, workspace_id, channel_id, type, title, body, data)
    SELECT $5::uuid,
           $2::uuid,
           $3::uuid,
           'invite',
           CASE WHEN $6 = 'video' THEN 'Cuộc gọi video đến' ELSE 'Cuộc gọi thoại đến' END,
           caller.display_name || ' đang gọi cho bạn',
           $7::jsonb || jsonb_build_object(
             'title', CASE WHEN $6 = 'video' THEN 'Cuộc gọi video đến' ELSE 'Cuộc gọi thoại đến' END,
             'body', caller.display_name || ' đang gọi cho bạn',
             'caller_name', caller.display_name
           )
    FROM caller
    LEFT JOIN notification_preferences np
      ON np.user_id = $5::uuid AND np.workspace_id = $2::uuid
    WHERE COALESCE(np.call_ringing, true)
      AND COALESCE(np.mode, 'all') <> 'muted'
      AND NOT EXISTS (
        SELECT 1 FROM notifications n
        WHERE n.user_id = $5::uuid AND n.data->>'event_id' = $1
      )
    RETURNING id::text, title, body
)
SELECT id, title, body FROM inserted
`, "call:"+params.CallID+":invite", params.WorkspaceID, params.ChannelID, params.InitiatorUserID, params.TargetUserID, params.Mode, string(data)).Scan(&notificationID, &title, &body)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}
	payload["title"] = title
	payload["body"] = body
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return r.createJob(ctx, notificationID, params.WorkspaceID, params.TargetUserID, encoded)
}

func (r *Repository) UpdateCallNotification(ctx context.Context, params notificationsapp.CallNotificationParams) error {
	if params.Status != "rejected" && params.Status != "cancelled" && params.Status != "ended" && params.Status != "missed" {
		return nil
	}
	shouldPush := params.Status == "cancelled" || params.Status == "missed"
	eventID := "call:" + params.CallID + ":" + params.Status
	var notificationID string
	var payload string
	err := r.pool.QueryRow(ctx, `
WITH caller AS (
    SELECT COALESCE(NULLIF(trim(display_name), ''), username::text, 'WebTui') AS display_name
    FROM users
    WHERE id = $4::uuid
), updated AS (
    UPDATE notifications n
    SET title = CASE
          WHEN $6 IN ('cancelled', 'missed') THEN 'Cuộc gọi nhỡ'
          WHEN $6 = 'ended' THEN 'Cuộc gọi đã kết thúc'
          ELSE 'Cuộc gọi đã từ chối'
        END,
        body = caller.display_name,
        read_at = CASE WHEN $6 IN ('rejected', 'ended') THEN COALESCE(n.read_at, now()) ELSE n.read_at END,
        data = n.data || jsonb_build_object(
          'event_id', $1,
          'event_type', 'call_ended',
          'target_type', 'call',
          'status', $6,
          'title', CASE
            WHEN $6 IN ('cancelled', 'missed') THEN 'Cuộc gọi nhỡ'
            WHEN $6 = 'ended' THEN 'Cuộc gọi đã kết thúc'
            ELSE 'Cuộc gọi đã từ chối'
          END,
          'body', caller.display_name,
          'tag', 'call-' || $2
        )
    FROM caller
    WHERE n.user_id = $5::uuid
      AND n.workspace_id = $3::uuid
      AND n.data->>'call_id' = $2
    RETURNING n.id::text, n.data::text
)
SELECT id, data FROM updated
`, eventID, params.CallID, params.WorkspaceID, params.InitiatorUserID, params.TargetUserID, params.Status).Scan(&notificationID, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if !shouldPush {
		return nil
	}
	return r.createJobOnce(ctx, notificationID, params.WorkspaceID, params.TargetUserID, []byte(payload), eventID)
}

func (r *Repository) GetPreference(ctx context.Context, zoneID string, userID string, workspaceID string) (notificationsdomain.NotificationPreference, error) {
	row := r.pool.QueryRow(ctx, `
WITH member AS (
    SELECT $1::uuid AS user_id, $2::uuid AS workspace_id
    WHERE EXISTS (
        SELECT 1
        FROM workspace_members wm
        JOIN workspaces w
          ON w.id = wm.workspace_id
         AND w.zone_id = $3::uuid
         AND w.status = 'active'
         AND w.deleted_at IS NULL
        WHERE wm.user_id = $1::uuid
          AND wm.workspace_id = $2::uuid
          AND wm.status IN ('active', 'muted')
    )
)
SELECT m.user_id::text, m.workspace_id::text,
       COALESCE(np.mode, 'all'),
       COALESCE(np.preview, true),
       COALESCE(np.quiet_hours, false),
       COALESCE(np.quiet_start::text, '22:00'),
       COALESCE(np.quiet_end::text, '07:00'),
       COALESCE(np.sound, true),
       COALESCE(np.vibrate, true),
       COALESCE(np.call_ringing, true),
       COALESCE(np.badge_enabled, true),
       COALESCE(np.created_at, now()),
       COALESCE(np.updated_at, now())
FROM member m
LEFT JOIN notification_preferences np
  ON np.user_id = m.user_id AND np.workspace_id = m.workspace_id
`, userID, workspaceID, zoneID)
	return scanNotificationPreference(row)
}

func (r *Repository) ListForUser(ctx context.Context, params notificationsapp.ListParams) ([]notificationsdomain.Notification, error) {
	rows, err := r.pool.Query(ctx, `
SELECT n.id::text, n.user_id::text, n.workspace_id::text, n.channel_id::text, n.message_id::text,
       n.type, n.title, n.body, n.data::text, n.read_at, n.delivered_at, n.created_at
FROM notifications n
JOIN workspaces w
  ON w.id = n.workspace_id
 AND w.zone_id = $2::uuid
 AND w.status = 'active'
 AND w.deleted_at IS NULL
WHERE n.user_id = $1::uuid
  AND ($3 = '' OR n.workspace_id = NULLIF($3, '')::uuid)
ORDER BY n.created_at DESC
LIMIT $4
`, params.UserID, params.ZoneID, params.WorkspaceID, params.Limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notifications := make([]notificationsdomain.Notification, 0)
	for rows.Next() {
		notification, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		notifications = append(notifications, notification)
	}
	return notifications, rows.Err()
}

func (r *Repository) MarkRead(ctx context.Context, zoneID string, userID string, notificationID string) (notificationsdomain.Notification, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE notifications
SET read_at = COALESCE(read_at, now())
WHERE id = $3::uuid
  AND user_id = $2::uuid
  AND EXISTS (
      SELECT 1
      FROM workspaces w
      WHERE w.id = notifications.workspace_id
        AND w.zone_id = $1::uuid
        AND w.status = 'active'
        AND w.deleted_at IS NULL
  )
RETURNING id::text, user_id::text, workspace_id::text, channel_id::text, message_id::text,
          type, title, body, data::text, read_at, delivered_at, created_at
`, zoneID, userID, notificationID)
	return scanNotification(row)
}

func (r *Repository) MarkAllRead(ctx context.Context, zoneID string, userID string, workspaceID string) error {
	_, err := r.pool.Exec(ctx, `
UPDATE notifications
SET read_at = COALESCE(read_at, now())
WHERE user_id = $2::uuid
  AND ($3 = '' OR workspace_id = NULLIF($3, '')::uuid)
  AND EXISTS (
      SELECT 1
      FROM workspaces w
      WHERE w.id = notifications.workspace_id
        AND w.zone_id = $1::uuid
        AND w.status = 'active'
        AND w.deleted_at IS NULL
  )
  AND read_at IS NULL
`, zoneID, userID, workspaceID)
	return err
}

func (r *Repository) UpsertPreference(ctx context.Context, zoneID string, preference notificationsdomain.NotificationPreference) (notificationsdomain.NotificationPreference, error) {
	row := r.pool.QueryRow(ctx, `
WITH member AS (
    SELECT $1::uuid AS user_id, $2::uuid AS workspace_id
    WHERE EXISTS (
        SELECT 1
        FROM workspace_members wm
        JOIN workspaces w
          ON w.id = wm.workspace_id
         AND w.zone_id = $12::uuid
         AND w.status = 'active'
         AND w.deleted_at IS NULL
        WHERE wm.user_id = $1::uuid
          AND wm.workspace_id = $2::uuid
          AND wm.status IN ('active', 'muted')
    )
),
upserted AS (
    INSERT INTO notification_preferences (user_id, workspace_id, mode, preview, quiet_hours, quiet_start, quiet_end, sound, vibrate, call_ringing, badge_enabled)
    SELECT user_id, workspace_id, $3, $4, $5, $6, $7, $8, $9, $10, $11
    FROM member
    ON CONFLICT (user_id, workspace_id) DO UPDATE SET
        mode = EXCLUDED.mode,
        preview = EXCLUDED.preview,
        quiet_hours = EXCLUDED.quiet_hours,
        quiet_start = EXCLUDED.quiet_start,
        quiet_end = EXCLUDED.quiet_end,
        sound = EXCLUDED.sound,
        vibrate = EXCLUDED.vibrate,
        call_ringing = EXCLUDED.call_ringing,
        badge_enabled = EXCLUDED.badge_enabled
    RETURNING user_id::text, workspace_id::text, mode, preview, quiet_hours, quiet_start::text, quiet_end::text,
              sound, vibrate, call_ringing, badge_enabled, created_at, updated_at
)
SELECT user_id, workspace_id, mode, preview, quiet_hours, quiet_start, quiet_end,
       sound, vibrate, call_ringing, badge_enabled, created_at, updated_at
FROM upserted
`, preference.UserID, preference.WorkspaceID, preference.Mode, preference.Preview, preference.QuietHours, preference.QuietStart, preference.QuietEnd,
		preference.Sound, preference.Vibrate, preference.CallRinging, preference.BadgeEnabled, zoneID)
	return scanNotificationPreference(row)
}

func (r *Repository) GetChannelPreference(ctx context.Context, zoneID string, userID string, workspaceID string, channelID string) (notificationsdomain.ChannelPreference, error) {
	row := r.pool.QueryRow(ctx, `
WITH member AS (
    SELECT $1::uuid AS user_id, $2::uuid AS workspace_id, $3::uuid AS channel_id
    WHERE EXISTS (
        SELECT 1
        FROM channel_members cm
        JOIN channels c ON c.id = cm.channel_id
        JOIN workspaces w
          ON w.id = c.workspace_id
         AND w.zone_id = $4::uuid
         AND w.status = 'active'
         AND w.deleted_at IS NULL
        WHERE cm.user_id = $1::uuid
          AND c.workspace_id = $2::uuid
          AND c.id = $3::uuid
          AND cm.status IN ('active', 'muted')
          AND c.deleted_at IS NULL
    )
)
SELECT m.user_id::text, m.workspace_id::text, m.channel_id::text,
       COALESCE(ncp.mode, 'all'),
       ncp.muted_until,
       COALESCE(ncp.created_at, now()),
       COALESCE(ncp.updated_at, now())
FROM member m
LEFT JOIN notification_channel_preferences ncp
  ON ncp.user_id = m.user_id AND ncp.workspace_id = m.workspace_id AND ncp.channel_id = m.channel_id
`, userID, workspaceID, channelID, zoneID)
	return scanChannelPreference(row)
}

func (r *Repository) UpsertChannelPreference(ctx context.Context, zoneID string, preference notificationsdomain.ChannelPreference) (notificationsdomain.ChannelPreference, error) {
	row := r.pool.QueryRow(ctx, `
WITH member AS (
    SELECT $1::uuid AS user_id, $2::uuid AS workspace_id, $3::uuid AS channel_id
    WHERE EXISTS (
        SELECT 1
        FROM channel_members cm
        JOIN channels c ON c.id = cm.channel_id
        JOIN workspaces w
          ON w.id = c.workspace_id
         AND w.zone_id = $6::uuid
         AND w.status = 'active'
         AND w.deleted_at IS NULL
        WHERE cm.user_id = $1::uuid
          AND c.workspace_id = $2::uuid
          AND c.id = $3::uuid
          AND cm.status IN ('active', 'muted')
          AND c.deleted_at IS NULL
    )
),
upserted AS (
    INSERT INTO notification_channel_preferences (user_id, workspace_id, channel_id, mode, muted_until)
    SELECT user_id, workspace_id, channel_id, $4, $5
    FROM member
    ON CONFLICT (user_id, workspace_id, channel_id) DO UPDATE SET
        mode = EXCLUDED.mode,
        muted_until = EXCLUDED.muted_until
    RETURNING user_id::text, workspace_id::text, channel_id::text, mode, muted_until, created_at, updated_at
)
SELECT user_id, workspace_id, channel_id, mode, muted_until, created_at, updated_at
FROM upserted
`, preference.UserID, preference.WorkspaceID, preference.ChannelID, preference.Mode, preference.MutedUntil, zoneID)
	return scanChannelPreference(row)
}

func (r *Repository) ProcessPendingJobs(ctx context.Context, limit int) (int, error) {
	rows, err := r.pool.Query(ctx, `
WITH picked AS (
    SELECT id
    FROM notification_jobs
    WHERE status = 'pending'
       OR (status = 'failed' AND (next_attempt_at IS NULL OR next_attempt_at <= now()))
    ORDER BY created_at ASC
    LIMIT $1
    FOR UPDATE SKIP LOCKED
),
updated AS (
    UPDATE notification_jobs nj
    SET status = 'processing',
        attempt_count = attempt_count + 1,
        error = NULL
    FROM picked
    WHERE nj.id = picked.id
    RETURNING nj.id::text, nj.notification_id::text, nj.user_id::text, nj.channel, nj.payload::text
)
SELECT id, notification_id, user_id, channel, payload FROM updated
`, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type pendingJob struct {
		id             string
		notificationID string
		userID         string
		channel        string
		payload        map[string]any
	}
	jobs := make([]pendingJob, 0)
	for rows.Next() {
		var job pendingJob
		var payload string
		if err := rows.Scan(&job.id, &job.notificationID, &job.userID, &job.channel, &payload); err != nil {
			return 0, err
		}
		if err := json.Unmarshal([]byte(payload), &job.payload); err != nil {
			_ = r.markJobFailed(ctx, job.id, err)
			continue
		}
		jobs = append(jobs, job)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	rows.Close()

	processed := 0
	for _, job := range jobs {
		if err := r.deliverJob(ctx, job.userID, job.channel, job.payload); err != nil {
			_ = r.markJobFailed(ctx, job.id, err)
			continue
		}
		if err := r.markJobSent(ctx, job.id, job.notificationID); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (r *Repository) deliverJob(ctx context.Context, userID string, channel string, payload map[string]any) error {
	if channel != "push" {
		return nil
	}
	if r.pushSender == nil || !r.pushSender.Enabled() {
		return errors.New("FCM push sender is not configured")
	}
	rows, err := r.pool.Query(ctx, `
SELECT push_token
FROM push_devices
WHERE user_id = $1::uuid
  AND status = 'active'
  AND push_provider = 'fcm'
  AND notification_permission IN ('granted', 'provisional')
  AND push_token IS NOT NULL
  AND length(trim(push_token)) > 0
`, userID)
	if err != nil {
		return err
	}
	defer rows.Close()

	tokens := make([]string, 0)
	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err != nil {
			return err
		}
		tokens = append(tokens, token)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, token := range tokens {
		if err := r.pushSender.Send(ctx, token, payload); err != nil {
			return fmt.Errorf("send FCM notification: %w", err)
		}
	}
	return nil
}

func (r *Repository) markJobSent(ctx context.Context, jobID string, notificationID string) error {
	_, err := r.pool.Exec(ctx, `
WITH sent AS (
    UPDATE notification_jobs
    SET status = 'sent', sent_at = now(), next_attempt_at = NULL, error = NULL
    WHERE id = $1::uuid
)
UPDATE notifications
SET delivered_at = COALESCE(delivered_at, now())
WHERE id = $2::uuid
`, jobID, notificationID)
	return err
}

func (r *Repository) markJobFailed(ctx context.Context, jobID string, cause error) error {
	reason := "push delivery failed"
	if cause != nil {
		reason = cause.Error()
	}
	_, err := r.pool.Exec(ctx, `
UPDATE notification_jobs
SET status = CASE WHEN attempt_count >= 5 THEN 'dead' ELSE 'failed' END,
    next_attempt_at = CASE WHEN attempt_count >= 5 THEN NULL ELSE now() + make_interval(secs => LEAST(300, attempt_count * 30)) END,
    error = left($2, 1000)
WHERE id = $1::uuid
`, jobID, reason)
	return err
}

func (r *Repository) createJob(ctx context.Context, notificationID string, workspaceID string, userID string, payload []byte) error {
	_, err := r.pool.Exec(ctx, `
INSERT INTO notification_jobs (notification_id, workspace_id, user_id, channel, payload)
VALUES ($1::uuid, $2::uuid, $3::uuid, 'push', $4::jsonb)
`, notificationID, workspaceID, userID, string(payload))
	return err
}

func (r *Repository) createJobOnce(ctx context.Context, notificationID string, workspaceID string, userID string, payload []byte, eventID string) error {
	_, err := r.pool.Exec(ctx, `
INSERT INTO notification_jobs (notification_id, workspace_id, user_id, channel, payload)
SELECT $1::uuid, $2::uuid, $3::uuid, 'push', $4::jsonb
WHERE NOT EXISTS (
    SELECT 1 FROM notification_jobs
    WHERE notification_id = $1::uuid AND payload->>'event_id' = $5
)
`, notificationID, workspaceID, userID, string(payload), eventID)
	return err
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanNotificationPreference(row rowScanner) (notificationsdomain.NotificationPreference, error) {
	var preference notificationsdomain.NotificationPreference
	if err := row.Scan(
		&preference.UserID,
		&preference.WorkspaceID,
		&preference.Mode,
		&preference.Preview,
		&preference.QuietHours,
		&preference.QuietStart,
		&preference.QuietEnd,
		&preference.Sound,
		&preference.Vibrate,
		&preference.CallRinging,
		&preference.BadgeEnabled,
		&preference.CreatedAt,
		&preference.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notificationsdomain.NotificationPreference{}, notificationsdomain.ErrNotificationPreferenceUnavailable
		}
		return notificationsdomain.NotificationPreference{}, err
	}
	return preference, nil
}

func scanChannelPreference(row rowScanner) (notificationsdomain.ChannelPreference, error) {
	var preference notificationsdomain.ChannelPreference
	var mutedUntil sql.NullTime
	if err := row.Scan(
		&preference.UserID,
		&preference.WorkspaceID,
		&preference.ChannelID,
		&preference.Mode,
		&mutedUntil,
		&preference.CreatedAt,
		&preference.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notificationsdomain.ChannelPreference{}, notificationsdomain.ErrNotificationPreferenceUnavailable
		}
		return notificationsdomain.ChannelPreference{}, err
	}
	preference.MutedUntil = nullTimePtr(mutedUntil)
	return preference, nil
}

func scanNotification(row rowScanner) (notificationsdomain.Notification, error) {
	var notification notificationsdomain.Notification
	var workspaceID sql.NullString
	var channelID sql.NullString
	var messageID sql.NullString
	var data string
	var readAt sql.NullTime
	var deliveredAt sql.NullTime
	if err := row.Scan(
		&notification.ID,
		&notification.UserID,
		&workspaceID,
		&channelID,
		&messageID,
		&notification.Type,
		&notification.Title,
		&notification.Body,
		&data,
		&readAt,
		&deliveredAt,
		&notification.CreatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notificationsdomain.Notification{}, notificationsdomain.ErrNotificationNotFound
		}
		return notificationsdomain.Notification{}, err
	}
	notification.WorkspaceID = nullStringPtr(workspaceID)
	notification.ChannelID = nullStringPtr(channelID)
	notification.MessageID = nullStringPtr(messageID)
	notification.Data = []byte(data)
	notification.ReadAt = nullTimePtr(readAt)
	notification.DeliveredAt = nullTimePtr(deliveredAt)
	return notification, nil
}

func uniqueMentions(ids []string, senderID string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" || id == senderID || seen[id] {
			continue
		}
		seen[id] = true
		result = append(result, id)
	}
	return result
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
