CREATE UNIQUE INDEX IF NOT EXISTS messages_client_message_id_idx
ON messages (
    workspace_id,
    channel_id,
    sender_id,
    ((metadata ->> 'client_message_id'))
)
WHERE deleted_at IS NULL
  AND sender_id IS NOT NULL
  AND metadata ? 'client_message_id'
  AND length(trim(metadata ->> 'client_message_id')) > 0;
