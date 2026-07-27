DROP TABLE IF EXISTS federated_conversation_invites;
DROP TABLE IF EXISTS workspace_talk_integrations;
DROP TABLE IF EXISTS channel_recording_consents;
DROP TABLE IF EXISTS channel_recordings;
DROP TABLE IF EXISTS channel_recording_policies;
DROP TABLE IF EXISTS channel_breakout_broadcasts;

ALTER TABLE channel_breakout_rooms
    DROP COLUMN IF EXISTS sequence,
    DROP COLUMN IF EXISTS started_at,
    DROP COLUMN IF EXISTS allow_self_select,
    DROP COLUMN IF EXISTS assignment_mode;
ALTER TABLE channel_breakout_rooms
    DROP CONSTRAINT IF EXISTS channel_breakout_rooms_status_check;
UPDATE channel_breakout_rooms SET status = 'open' WHERE status IN ('prepared', 'active');
ALTER TABLE channel_breakout_rooms
    ALTER COLUMN status SET DEFAULT 'open';
ALTER TABLE channel_breakout_rooms
    ADD CONSTRAINT channel_breakout_rooms_status_check
    CHECK (status IN ('open', 'closed'));

DROP TABLE IF EXISTS channel_voice_rooms;
DROP TABLE IF EXISTS channel_meetings;
DROP TABLE IF EXISTS file_upload_parts;
DROP TABLE IF EXISTS file_upload_sessions;
DROP TABLE IF EXISTS message_thread_subscriptions;
DROP TABLE IF EXISTS message_threads;
DROP TABLE IF EXISTS message_reminders;
DROP TABLE IF EXISTS scheduled_messages;

DROP INDEX IF EXISTS message_pins_active_expiry_idx;
ALTER TABLE message_pins DROP COLUMN IF EXISTS expires_at;

DROP INDEX IF EXISTS notification_channel_preferences_tags_gin_idx;
DROP INDEX IF EXISTS notification_channel_preferences_user_archived_idx;
DROP INDEX IF EXISTS notification_channel_preferences_user_important_idx;
ALTER TABLE notification_channel_preferences
    DROP CONSTRAINT IF EXISTS notification_channel_preferences_tags_array_check,
    DROP COLUMN IF EXISTS archived_at,
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS compact,
    DROP COLUMN IF EXISTS important,
    DROP COLUMN IF EXISTS sensitive;

UPDATE notifications
SET type = 'system'
WHERE type IN ('reminder', 'meeting', 'recording');
ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS notifications_type_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_type_check
    CHECK (type IN ('mention', 'message', 'invite', 'system', 'webhook', 'bot'));
