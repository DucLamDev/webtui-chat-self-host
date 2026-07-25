-- Ensure every direct conversation participant is also a channel member.
-- Older data can miss these rows when accounts/conversations were created
-- before the direct-message repair path existed.

INSERT INTO channel_members (channel_id, user_id, status, joined_at)
SELECT DISTINCT dc.channel_id, dcm.user_id, 'active', now()
FROM direct_conversations dc
JOIN direct_conversation_members dcm ON dcm.direct_conversation_id = dc.id
JOIN channels c ON c.id = dc.channel_id AND c.deleted_at IS NULL AND c.status = 'active'
JOIN workspace_members wm
  ON wm.workspace_id = dc.workspace_id
 AND wm.user_id = dcm.user_id
 AND wm.status = 'active'
WHERE dc.archived_at IS NULL
ON CONFLICT (channel_id, user_id)
DO UPDATE SET
    status = 'active',
    joined_at = COALESCE(channel_members.joined_at, EXCLUDED.joined_at)
WHERE channel_members.status IN ('invited', 'left', 'removed');
