WITH guide(slug, body) AS (
    VALUES
        ('gia-han', 'Kiểm tra dịch vụ sắp hết hạn
Email: khach@example.com
Số ngày: 30
Loại dịch vụ: VPS'),
        ('ke-toan', 'Tạo QR nạp ví
Email: khach@example.com
Số tiền: 200000'),
        ('ticket', 'Tạo ticket hỗ trợ cho khach@example.com
VPS #1234 bị mất kết nối, ping timeout và port 22 không truy cập được.'),
        ('server-alert', 'Server: vps-01
Lỗi: Mất ping 3 phút
IP: 192.0.2.10
Mức độ: critical')
),
source_targets AS (
    SELECT DISTINCT
        c.workspace_id,
        c.id AS channel_id,
        c.id AS source_channel_id,
        c.slug::text AS source_slug,
        COALESCE(c.created_by, w.owner_id) AS actor_user_id,
        guide.body
    FROM channels c
    JOIN workspaces w
      ON w.id = c.workspace_id
    JOIN guide
      ON guide.slug = c.slug::text
    WHERE c.deleted_at IS NULL
      AND c.status = 'active'
      AND EXISTS (
          SELECT 1
          FROM bot_installations bi
          WHERE bi.workspace_id = c.workspace_id
            AND bi.channel_id = c.id
            AND bi.status = 'active'
      )
),
session_targets AS (
    SELECT DISTINCT
        session.workspace_id,
        session.id AS channel_id,
        source.id AS source_channel_id,
        source.slug::text AS source_slug,
        COALESCE(NULLIF(session.settings->>'bot_session_user_id', '')::uuid, session.created_by, w.owner_id) AS actor_user_id,
        guide.body
    FROM channels session
    JOIN workspaces w
      ON w.id = session.workspace_id
    JOIN channels source
      ON source.workspace_id = session.workspace_id
     AND source.id::text = session.settings->>'bot_source_channel_id'
     AND source.deleted_at IS NULL
    JOIN guide
      ON guide.slug = source.slug::text
    WHERE session.deleted_at IS NULL
      AND session.status = 'active'
      AND session.settings @> '{"bot_session":true}'::jsonb
),
targets AS (
    SELECT * FROM source_targets
    UNION
    SELECT * FROM session_targets
),
inserted AS (
    INSERT INTO messages (workspace_id, channel_id, sender_id, kind, body, metadata)
    SELECT
        targets.workspace_id,
        targets.channel_id,
        targets.actor_user_id,
        'text',
        targets.body,
        jsonb_build_object(
            'seed', 'bot_channel_guide',
            'source_channel_id', targets.source_channel_id::text,
            'source_slug', targets.source_slug
        )
    FROM targets
    WHERE targets.actor_user_id IS NOT NULL
      AND NOT EXISTS (
          SELECT 1
          FROM messages m
          WHERE m.workspace_id = targets.workspace_id
            AND m.channel_id = targets.channel_id
            AND m.metadata @> jsonb_build_object(
                'seed', 'bot_channel_guide',
                'source_channel_id', targets.source_channel_id::text
            )
            AND m.deleted_at IS NULL
      )
    RETURNING id, workspace_id, channel_id, sender_id
),
guide_messages AS (
    SELECT m.id, m.workspace_id, m.channel_id, targets.actor_user_id AS pinned_by
    FROM targets
    JOIN messages m
      ON m.workspace_id = targets.workspace_id
     AND m.channel_id = targets.channel_id
     AND m.metadata @> jsonb_build_object(
         'seed', 'bot_channel_guide',
         'source_channel_id', targets.source_channel_id::text
     )
     AND m.deleted_at IS NULL
    WHERE targets.actor_user_id IS NOT NULL
    UNION
    SELECT id, workspace_id, channel_id, sender_id AS pinned_by
    FROM inserted
    WHERE sender_id IS NOT NULL
)
INSERT INTO message_pins (workspace_id, channel_id, message_id, pinned_by)
SELECT workspace_id, channel_id, id, pinned_by
FROM guide_messages
ON CONFLICT (workspace_id, channel_id, message_id)
DO UPDATE SET pinned_by = EXCLUDED.pinned_by,
              created_at = message_pins.created_at;
