DROP INDEX IF EXISTS channels_bot_private_session_uidx;

DELETE FROM permissions WHERE code = 'order.payment_request';

UPDATE channels
SET settings = settings - 'bot_session_mode',
    updated_at = now()
WHERE settings->>'bot_session_mode' = 'private';

INSERT INTO channel_members (channel_id, user_id, status)
SELECT c.id, wm.user_id, 'active'
FROM channels c
JOIN workspace_members wm
  ON wm.workspace_id = c.workspace_id
 AND wm.status = 'active'
WHERE c.type = 'public'
  AND c.status = 'active'
  AND c.deleted_at IS NULL
ON CONFLICT (channel_id, user_id) DO UPDATE SET status = 'active';

INSERT INTO channel_members (channel_id, user_id, status)
SELECT c.id, c.created_by, 'active'
FROM channels c
WHERE c.created_by IS NOT NULL
  AND c.settings @> '{"system_default":true}'::jsonb
  AND c.deleted_at IS NULL
ON CONFLICT (channel_id, user_id) DO UPDATE SET status = 'active';

UPDATE channels
SET status = 'deleted', deleted_at = now()
WHERE settings @> '{"bot_session":true}'::jsonb
  AND deleted_at IS NULL;
