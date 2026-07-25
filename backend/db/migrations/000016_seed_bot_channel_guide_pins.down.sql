DELETE FROM message_pins mp
USING messages m
WHERE mp.workspace_id = m.workspace_id
  AND mp.channel_id = m.channel_id
  AND mp.message_id = m.id
  AND m.metadata->>'seed' = 'bot_channel_guide';

DELETE FROM messages
WHERE metadata->>'seed' = 'bot_channel_guide';
