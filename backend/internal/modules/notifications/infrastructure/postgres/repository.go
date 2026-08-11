package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	notificationsapp "github.com/duclamdev/application-chat/backend/internal/modules/notifications/application"
	notificationsdomain "github.com/duclamdev/application-chat/backend/internal/modules/notifications/domain"
	"github.com/duclamdev/application-chat/backend/internal/modules/notifications/infrastructure/pusherror"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool          *pgxpool.Pool
	pushSenders   map[string]PushSender
	webPushSender WebPushSender
}

type PushSender interface {
	Provider() string
	Enabled() bool
	Send(ctx context.Context, token string, payload map[string]any) error
}

type WebPushSender interface {
	Enabled() bool
	Send(ctx context.Context, endpoint string, p256dh string, auth string, payload map[string]any) error
}

type nativeDestination struct {
	id       string
	deviceID string
	provider string
	token    string
	zoneID   string
}

var nativeInstanceIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func NewRepository(pool *pgxpool.Pool, pushSenders ...PushSender) *Repository {
	repository := &Repository{pool: pool, pushSenders: make(map[string]PushSender)}
	for _, sender := range pushSenders {
		if sender == nil {
			continue
		}
		provider := strings.ToLower(strings.TrimSpace(sender.Provider()))
		if provider != "" {
			repository.pushSenders[provider] = sender
		}
	}
	return repository
}

func (r *Repository) SetWebPushSender(sender WebPushSender) {
	if r != nil {
		r.webPushSender = sender
	}
}

func (r *Repository) CreateMentionNotifications(ctx context.Context, params notificationsapp.MentionParams) error {
	for _, userID := range uniqueMentions(params.MentionedUserIDs, params.SenderID) {
		data, err := json.Marshal(map[string]any{
			"event_id":     params.EventID,
			"event_type":   "mention",
			"target_type":  "message",
			"workspace_id": params.WorkspaceID,
			"channel_id":   params.ChannelID,
			"message_id":   params.MessageID,
			"sender_id":    params.SenderID,
			"title":        "Bạn được nhắc trong một tin nhắn",
			"body":         "",
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
    SELECT m.id, m.workspace_id, m.channel_id, m.sender_id, m.kind, m.body, m.metadata,
           COALESCE(NULLIF(trim(u.display_name), ''), u.username::text, 'Người dùng') AS sender_name
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
           COALESCE(np.preview, true) AND NOT COALESCE(ncp.sensitive, false) AS preview,
           COALESCE(ncp.important, false) AS important,
           COALESCE(np.quiet_hours, false) AS quiet_hours,
           COALESCE(np.quiet_start, '22:00'::time) AS quiet_start,
           COALESCE(np.quiet_end, '07:00'::time) AS quiet_end
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
      AND COALESCE(s.metadata->>'silent', 'false') <> 'true'
      AND (
          COALESCE(np.mode, 'all') = 'all'
          OR (COALESCE(np.mode, 'all') = 'mentions' AND cm.user_id = ANY($6::uuid[]))
      )
      AND (
          COALESCE(ncp.important, false)
          OR NOT COALESCE(np.quiet_hours, false)
          OR CASE
               WHEN COALESCE(np.quiet_start, '22:00'::time) <= COALESCE(np.quiet_end, '07:00'::time)
                 THEN localtime NOT BETWEEN COALESCE(np.quiet_start, '22:00'::time)
                                        AND COALESCE(np.quiet_end, '07:00'::time)
               ELSE localtime < COALESCE(np.quiet_start, '22:00'::time)
                 AND localtime > COALESCE(np.quiet_end, '07:00'::time)
             END
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
    SELECT COALESCE(NULLIF(trim(display_name), ''), username::text, 'Người dùng') AS display_name
    FROM users
    WHERE id = $4::uuid
), organization AS (
    SELECT z.name,
           COALESCE(z.metadata #>> '{branding,logo_url}', '') AS logo_url
    FROM workspaces w
    JOIN zones z ON z.id = w.zone_id AND z.deleted_at IS NULL
    WHERE w.id = $2::uuid
      AND w.deleted_at IS NULL
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
             'caller_name', caller.display_name,
             'app_name', organization.name,
             'logo_url', organization.logo_url
           )
    FROM caller
    CROSS JOIN organization
    LEFT JOIN notification_preferences np
      ON np.user_id = $5::uuid AND np.workspace_id = $2::uuid
    WHERE COALESCE(np.call_ringing, true)
      AND COALESCE(np.mode, 'all') <> 'muted'
      AND NOT EXISTS (
        SELECT 1 FROM notifications n
        WHERE n.user_id = $5::uuid AND n.data->>'event_id' = $1
      )
    RETURNING id::text, title, body, data::text
)
SELECT id, title, body, data FROM inserted
`, "call:"+params.CallID+":invite", params.WorkspaceID, params.ChannelID, params.InitiatorUserID, params.TargetUserID, params.Mode, string(data)).Scan(&notificationID, &title, &body, &data)
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
    SELECT COALESCE(NULLIF(trim(display_name), ''), username::text, 'Người dùng') AS display_name
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
       COALESCE(ncp.sensitive, false),
       COALESCE(ncp.important, false),
       COALESCE(ncp.compact, false),
       COALESCE(ncp.tags, '[]'::jsonb)::text,
       ncp.archived_at,
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
         AND w.zone_id = $11::uuid
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
    INSERT INTO notification_channel_preferences (
        user_id, workspace_id, channel_id, mode, muted_until,
        sensitive, important, compact, tags, archived_at
    )
    SELECT user_id, workspace_id, channel_id, $4, $5, $6, $7, $8, $9::jsonb, $10
    FROM member
    ON CONFLICT (user_id, workspace_id, channel_id) DO UPDATE SET
        mode = EXCLUDED.mode,
        muted_until = EXCLUDED.muted_until,
        sensitive = EXCLUDED.sensitive,
        important = EXCLUDED.important,
        compact = EXCLUDED.compact,
        tags = EXCLUDED.tags,
        archived_at = EXCLUDED.archived_at
    RETURNING user_id::text, workspace_id::text, channel_id::text, mode,
              muted_until, sensitive, important, compact, tags::text,
              archived_at, created_at, updated_at
)
SELECT user_id, workspace_id, channel_id, mode, muted_until, sensitive,
       important, compact, tags, archived_at, created_at, updated_at
FROM upserted
`, preference.UserID, preference.WorkspaceID, preference.ChannelID,
		preference.Mode, preference.MutedUntil, preference.Sensitive,
		preference.Important, preference.Compact, mustJSON(preference.Tags),
		preference.ArchivedAt, zoneID)
	return scanChannelPreference(row)
}

func (r *Repository) UpsertWebPushSubscription(ctx context.Context, params notificationsapp.WebPushSubscriptionParams) (notificationsdomain.WebPushSubscription, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return notificationsdomain.WebPushSubscription{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Serialize registration and quota trimming per account. Without this lock,
	// concurrent transactions can each miss the other's uncommitted endpoint
	// and leave more active subscriptions than the configured limit.
	if _, err := tx.Exec(ctx, `
SELECT pg_advisory_xact_lock(hashtextextended($1, 0))
`, params.ZoneID+":"+params.UserID); err != nil {
		return notificationsdomain.WebPushSubscription{}, err
	}

	var subscription notificationsdomain.WebPushSubscription
	var eligible bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM workspace_members member
    JOIN workspaces workspace
      ON workspace.id = member.workspace_id
     AND workspace.zone_id = $1::uuid
     AND workspace.status = 'active'
     AND workspace.deleted_at IS NULL
    WHERE member.user_id = $2::uuid
      AND member.workspace_id = $3::uuid
      AND member.status IN ('active', 'muted')
)
`, params.ZoneID, params.UserID, params.WorkspaceID).Scan(&eligible); err != nil {
		return notificationsdomain.WebPushSubscription{}, err
	}
	if !eligible {
		return notificationsdomain.WebPushSubscription{}, notificationsdomain.ErrNotificationPreferenceUnavailable
	}

	err = tx.QueryRow(ctx, `
INSERT INTO web_push_subscriptions (
    zone_id, user_id, endpoint, p256dh, auth_secret,
    expiration_time, status, last_seen_at, revoked_at
)
VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, 'active', statement_timestamp(), NULL)
ON CONFLICT (endpoint_hash) DO UPDATE SET
    endpoint = EXCLUDED.endpoint,
    p256dh = EXCLUDED.p256dh,
    auth_secret = EXCLUDED.auth_secret,
    expiration_time = EXCLUDED.expiration_time,
    status = 'active',
    last_seen_at = statement_timestamp(),
    revoked_at = NULL
WHERE web_push_subscriptions.zone_id = EXCLUDED.zone_id
  AND web_push_subscriptions.user_id = EXCLUDED.user_id
RETURNING id::text, user_id::text, created_at, updated_at
`, params.ZoneID, params.UserID, params.Endpoint, params.P256DH, params.Auth, params.ExpirationTime).Scan(
		&subscription.ID,
		&subscription.UserID,
		&subscription.CreatedAt,
		&subscription.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return notificationsdomain.WebPushSubscription{}, notificationsdomain.ErrWebPushSubscriptionConflict
	}
	if err != nil {
		return notificationsdomain.WebPushSubscription{}, err
	}

	maxSubscriptions := params.MaxSubscriptionsPerUser
	if maxSubscriptions <= 0 {
		maxSubscriptions = 10
	}
	_, err = tx.Exec(ctx, `
WITH stale AS (
    SELECT id
    FROM web_push_subscriptions
    WHERE zone_id = $1::uuid
      AND user_id = $2::uuid
      AND status = 'active'
    ORDER BY (id = $4::uuid) DESC, last_seen_at DESC, created_at DESC, id
    OFFSET $3
)
UPDATE web_push_subscriptions subscription
SET status = 'revoked', revoked_at = statement_timestamp()
FROM stale
WHERE subscription.id = stale.id
`, params.ZoneID, params.UserID, maxSubscriptions, subscription.ID)
	if err != nil {
		return notificationsdomain.WebPushSubscription{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return notificationsdomain.WebPushSubscription{}, err
	}
	return subscription, nil
}

func (r *Repository) RevokeWebPushSubscription(ctx context.Context, zoneID string, userID string, subscriptionID string) error {
	command, err := r.pool.Exec(ctx, `
UPDATE web_push_subscriptions
SET status = 'revoked', revoked_at = now()
WHERE id = $3::uuid
  AND zone_id = $1::uuid
  AND user_id = $2::uuid
  AND status = 'active'
`, zoneID, userID, subscriptionID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return notificationsdomain.ErrWebPushSubscriptionNotFound
	}
	return nil
}

func (r *Repository) ProcessPendingJobs(ctx context.Context, limit int) (int, error) {
	rows, err := r.pool.Query(ctx, `
WITH picked AS (
    SELECT id
    FROM notification_jobs
    WHERE status = 'pending'
       OR (status = 'failed' AND (next_attempt_at IS NULL OR next_attempt_at <= now()))
       OR (status = 'processing' AND updated_at <= now() - interval '5 minutes')
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
    RETURNING nj.id::text, nj.notification_id::text, nj.workspace_id::text,
              nj.user_id::text, nj.channel, nj.payload::text
)
SELECT id, notification_id, workspace_id, user_id, channel, payload FROM updated
`, limit)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	type pendingJob struct {
		id             string
		notificationID string
		workspaceID    string
		userID         string
		channel        string
		payload        map[string]any
	}
	jobs := make([]pendingJob, 0)
	for rows.Next() {
		var job pendingJob
		var payload string
		if err := rows.Scan(&job.id, &job.notificationID, &job.workspaceID, &job.userID, &job.channel, &payload); err != nil {
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
		if err := r.deliverJob(ctx, job.id, job.workspaceID, job.userID, job.channel, job.payload); err != nil {
			_ = r.markJobFailed(ctx, job.id, err)
			continue
		}
		if job.channel == "push" {
			delivered, err := r.hasAcceptedDestination(ctx, job.id)
			if err != nil {
				_ = r.markJobFailed(ctx, job.id, fmt.Errorf("inspect push delivery ledger: %w", err))
				continue
			}
			if !delivered {
				if err := r.markJobSkipped(ctx, job.id); err != nil {
					return processed, err
				}
				processed++
				continue
			}
		}
		if err := r.markJobSent(ctx, job.id, job.notificationID); err != nil {
			return processed, err
		}
		processed++
	}
	return processed, nil
}

func (r *Repository) deliverJob(ctx context.Context, jobID string, workspaceID string, userID string, channel string, payload map[string]any) error {
	if channel != "push" {
		return nil
	}
	webPushEnabled := r.webPushSender != nil && r.webPushSender.Enabled()
	if len(r.pushSenders) == 0 && !webPushEnabled {
		// Push is an explicit optional mode for self-hosted installations. The
		// durable in-app notification remains available without creating dead
		// jobs when the operator intentionally leaves all providers disabled.
		return nil
	}
	var deliveryErrors []error
	if len(r.pushSenders) > 0 {
		deliveryErrors = append(deliveryErrors, r.deliverNativeJob(ctx, jobID, workspaceID, userID, payload)...)
	}
	if webPushEnabled {
		if _, exists := payload["event_id"]; !exists {
			payload["event_id"] = "notification-job:" + jobID
		}
		deliveryErrors = append(deliveryErrors, r.deliverWebPushJob(ctx, jobID, workspaceID, userID, payload)...)
	}
	return errors.Join(deliveryErrors...)
}

func (r *Repository) deliverNativeJob(ctx context.Context, jobID string, workspaceID string, userID string, payload map[string]any) []error {
	rows, err := r.pool.Query(ctx, `
SELECT device.id::text, device.device_id, device.push_provider, device.push_token,
       workspace.zone_id::text
FROM push_devices device
JOIN workspaces workspace
  ON workspace.id = $2::uuid
 AND workspace.zone_id = device.zone_id
 AND workspace.deleted_at IS NULL
JOIN workspace_members member
  ON member.workspace_id = workspace.id
 AND member.user_id = device.user_id
 AND member.status IN ('active', 'muted')
WHERE device.user_id = $1::uuid
  AND device.workspace_id = $2::uuid
  AND device.status = 'active'
  AND device.push_provider IN ('fcm', 'apns')
  AND device.notification_permission IN ('granted', 'provisional')
  AND device.push_token IS NOT NULL
  AND length(trim(device.push_token)) > 0
  AND NOT EXISTS (
      SELECT 1
      FROM notification_job_deliveries delivered
      WHERE delivered.job_id = $3::uuid
        AND delivered.device_id = device.id
  )
`, userID, workspaceID, jobID)
	if err != nil {
		return []error{err}
	}
	defer rows.Close()

	destinations := make([]nativeDestination, 0)
	for rows.Next() {
		var item nativeDestination
		if err := rows.Scan(&item.id, &item.deviceID, &item.provider, &item.token, &item.zoneID); err != nil {
			return []error{err}
		}
		destinations = append(destinations, item)
	}
	if err := rows.Err(); err != nil {
		return []error{err}
	}
	eventType, _ := payload["event_type"].(string)
	apnsSender := r.pushSenders["apns"]
	destinations = selectNativeDestinations(
		destinations,
		eventType,
		apnsSender != nil && apnsSender.Enabled(),
	)
	var deliveryErrors []error
	for _, destination := range destinations {
		sender := r.pushSenders[strings.ToLower(strings.TrimSpace(destination.provider))]
		if sender == nil || !sender.Enabled() {
			// Providers are opt-in for self-hosters. A token for an intentionally
			// disabled provider is not a failed delivery destination.
			continue
		}
		providerPayload, err := nativeProviderPayload(payload, destination.zoneID)
		if err != nil {
			deliveryErrors = append(deliveryErrors, fmt.Errorf("prepare %s notification: %w", destination.provider, err))
			continue
		}
		if err := sender.Send(ctx, destination.token, providerPayload); err != nil {
			if pusherror.IsPermanent(err) {
				revoked, revokeErr := r.pool.Exec(ctx, `
UPDATE push_devices
SET status = 'revoked', revoked_at = now()
WHERE id = $1::uuid
  AND push_token = $2
`, destination.id, destination.token)
				if revokeErr != nil {
					deliveryErrors = append(deliveryErrors, fmt.Errorf("revoke invalid %s token: %w", destination.provider, revokeErr))
					continue
				}
				if revoked.RowsAffected() == 0 {
					deliveryErrors = append(deliveryErrors, fmt.Errorf("%s push token changed during delivery", destination.provider))
					continue
				}
				continue
			}
			deliveryErrors = append(deliveryErrors, fmt.Errorf("send %s notification: %w", destination.provider, err))
			continue
		}
		if err := r.markDestinationDelivered(ctx, jobID, destination.id, destination.provider); err != nil {
			deliveryErrors = append(deliveryErrors, fmt.Errorf("record delivered %s destination: %w", destination.provider, err))
		}
	}
	return deliveryErrors
}

func nativeProviderPayload(payload map[string]any, zoneID string) (map[string]any, error) {
	instanceID := strings.ToLower(strings.TrimSpace(zoneID))
	if !nativeInstanceIDPattern.MatchString(instanceID) {
		return nil, errors.New("destination zone identity is invalid")
	}
	cloned := make(map[string]any, len(payload)+1)
	for key, value := range payload {
		cloned[key] = value
	}
	// The workspace/device join is authoritative. A queued payload cannot select
	// another instance or suppress the instance identity delivered to mobile.
	cloned["instance_id"] = instanceID
	return cloned, nil
}

func selectNativeDestinations(destinations []nativeDestination, eventType string, apnsEnabled bool) []nativeDestination {
	callInvite := strings.EqualFold(strings.TrimSpace(eventType), "call_invite")
	voipDeviceIDs := make(map[string]struct{})
	if callInvite && apnsEnabled {
		for _, destination := range destinations {
			if strings.EqualFold(strings.TrimSpace(destination.provider), "apns") && strings.HasSuffix(destination.deviceID, ":voip") {
				voipDeviceIDs[strings.TrimSuffix(destination.deviceID, ":voip")] = struct{}{}
			}
		}
	}

	selected := make([]nativeDestination, 0, len(destinations))
	for _, destination := range destinations {
		provider := strings.ToLower(strings.TrimSpace(destination.provider))
		if provider == "apns" && (!callInvite || !apnsEnabled) {
			// APNs registrations are PushKit VoIP tokens and are never valid for
			// ordinary notifications. If APNs is unavailable, preserve paired FCM
			// as the call fallback.
			continue
		}
		if callInvite && provider == "fcm" {
			if _, pairedVoIP := voipDeviceIDs[destination.deviceID]; pairedVoIP {
				continue
			}
		}
		selected = append(selected, destination)
	}
	return selected
}

func (r *Repository) deliverWebPushJob(ctx context.Context, jobID string, workspaceID string, userID string, payload map[string]any) []error {
	rows, err := r.pool.Query(ctx, `
SELECT subscription.id::text, subscription.endpoint, subscription.p256dh, subscription.auth_secret
FROM web_push_subscriptions subscription
JOIN workspaces workspace
  ON workspace.id = $2::uuid
 AND workspace.zone_id = subscription.zone_id
 AND workspace.status = 'active'
 AND workspace.deleted_at IS NULL
JOIN workspace_members member
  ON member.workspace_id = workspace.id
 AND member.user_id = subscription.user_id
 AND member.status IN ('active', 'muted')
WHERE subscription.user_id = $1::uuid
  AND subscription.status = 'active'
  AND (subscription.expiration_time IS NULL OR subscription.expiration_time > now())
  AND NOT EXISTS (
      SELECT 1
      FROM notification_web_push_deliveries delivered
      WHERE delivered.job_id = $3::uuid
        AND delivered.subscription_id = subscription.id
  )
`, userID, workspaceID, jobID)
	if err != nil {
		return []error{err}
	}
	defer rows.Close()

	type destination struct {
		id       string
		endpoint string
		p256dh   string
		auth     string
	}
	destinations := make([]destination, 0)
	for rows.Next() {
		var item destination
		if err := rows.Scan(&item.id, &item.endpoint, &item.p256dh, &item.auth); err != nil {
			return []error{err}
		}
		destinations = append(destinations, item)
	}
	if err := rows.Err(); err != nil {
		return []error{err}
	}

	var deliveryErrors []error
	for _, destination := range destinations {
		err := r.webPushSender.Send(ctx, destination.endpoint, destination.p256dh, destination.auth, payload)
		if err != nil && pusherror.IsPermanent(err) {
			revoked, revokeErr := r.pool.Exec(ctx, `
UPDATE web_push_subscriptions
SET status = 'revoked', revoked_at = now()
WHERE id = $1::uuid
  AND endpoint = $2
`, destination.id, destination.endpoint)
			if revokeErr != nil {
				deliveryErrors = append(deliveryErrors, fmt.Errorf("revoke invalid Web Push subscription: %w", revokeErr))
				continue
			}
			if revoked.RowsAffected() == 0 {
				deliveryErrors = append(deliveryErrors, errors.New("Web Push subscription changed during delivery"))
				continue
			}
			continue
		}
		if err != nil {
			deliveryErrors = append(deliveryErrors, fmt.Errorf("send Web Push notification: %w", err))
			continue
		}
		if err := r.markWebDestinationDelivered(ctx, jobID, destination.id); err != nil {
			deliveryErrors = append(deliveryErrors, fmt.Errorf("record delivered Web Push subscription: %w", err))
		}
	}
	return deliveryErrors
}

func (r *Repository) markDestinationDelivered(ctx context.Context, jobID string, deviceID string, provider string) error {
	_, err := r.pool.Exec(ctx, `
INSERT INTO notification_job_deliveries (job_id, device_id, provider)
VALUES ($1::uuid, $2::uuid, $3)
ON CONFLICT (job_id, device_id) DO NOTHING
`, jobID, deviceID, strings.ToLower(strings.TrimSpace(provider)))
	return err
}

func (r *Repository) markWebDestinationDelivered(ctx context.Context, jobID string, subscriptionID string) error {
	_, err := r.pool.Exec(ctx, `
INSERT INTO notification_web_push_deliveries (job_id, subscription_id)
VALUES ($1::uuid, $2::uuid)
ON CONFLICT (job_id, subscription_id) DO NOTHING
`, jobID, subscriptionID)
	return err
}

func (r *Repository) hasAcceptedDestination(ctx context.Context, jobID string) (bool, error) {
	var delivered bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1 FROM notification_job_deliveries WHERE job_id = $1::uuid
    UNION ALL
    SELECT 1 FROM notification_web_push_deliveries WHERE job_id = $1::uuid
)
`, jobID).Scan(&delivered)
	return delivered, err
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

func (r *Repository) markJobSkipped(ctx context.Context, jobID string) error {
	_, err := r.pool.Exec(ctx, `
UPDATE notification_jobs
SET status = 'skipped', sent_at = NULL, next_attempt_at = NULL, error = NULL
WHERE id = $1::uuid
`, jobID)
	return err
}

func (r *Repository) markJobFailed(ctx context.Context, jobID string, cause error) error {
	reason := "push delivery failed"
	if cause != nil {
		reason = cause.Error()
	}
	terminal := pusherror.IsTerminal(cause) && !pusherror.HasRetryable(cause)
	deferred := pusherror.IsDeferred(cause) && !pusherror.HasNonDeferredRetryable(cause)
	deferredDelay := deferredRetryDelay(cause)
	deferredDelaySeconds := int64((deferredDelay + time.Second - 1) / time.Second)
	_, err := r.pool.Exec(ctx, `
UPDATE notification_jobs
SET attempt_count = CASE WHEN $4 THEN GREATEST(attempt_count - 1, 0) ELSE attempt_count END,
    status = CASE WHEN $3 OR (NOT $4 AND attempt_count >= 5) THEN 'dead' ELSE 'failed' END,
    next_attempt_at = CASE
        WHEN $3 OR (NOT $4 AND attempt_count >= 5) THEN NULL
        WHEN $4 THEN now() + make_interval(secs => $5::double precision)
        ELSE now() + make_interval(secs => LEAST(300, attempt_count * 30))
    END,
    error = left($2, 1000)
WHERE id = $1::uuid
`, jobID, reason, terminal, deferred, deferredDelaySeconds)
	return err
}

func deferredRetryDelay(cause error) time.Duration {
	const (
		minimum = 5 * time.Second
		maximum = 5 * time.Minute
	)
	delay, ok := pusherror.Delay(cause)
	if !ok || delay < minimum {
		return minimum
	}
	if delay > maximum {
		return maximum
	}
	return delay
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
	var archivedAt sql.NullTime
	var tags string
	if err := row.Scan(
		&preference.UserID,
		&preference.WorkspaceID,
		&preference.ChannelID,
		&preference.Mode,
		&mutedUntil,
		&preference.Sensitive,
		&preference.Important,
		&preference.Compact,
		&tags,
		&archivedAt,
		&preference.CreatedAt,
		&preference.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return notificationsdomain.ChannelPreference{}, notificationsdomain.ErrNotificationPreferenceUnavailable
		}
		return notificationsdomain.ChannelPreference{}, err
	}
	preference.MutedUntil = nullTimePtr(mutedUntil)
	preference.ArchivedAt = nullTimePtr(archivedAt)
	if err := json.Unmarshal([]byte(tags), &preference.Tags); err != nil {
		return notificationsdomain.ChannelPreference{}, err
	}
	if preference.Tags == nil {
		preference.Tags = []string{}
	}
	return preference, nil
}

func mustJSON(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}
	return string(encoded)
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
