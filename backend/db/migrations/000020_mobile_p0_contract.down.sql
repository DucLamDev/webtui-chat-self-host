DROP TABLE IF EXISTS bot_flow_runs;
DROP INDEX IF EXISTS bot_flows_single_published_uidx;
DROP TABLE IF EXISTS bot_flows;
DROP TABLE IF EXISTS bot_ai_configs;
DROP TABLE IF EXISTS call_signals;
DROP INDEX IF EXISTS call_sessions_client_call_uidx;
DROP TABLE IF EXISTS call_sessions;
DROP TABLE IF EXISTS mobile_sync_cursors;
DROP TABLE IF EXISTS notification_channel_preferences;

ALTER TABLE notification_preferences
    DROP COLUMN IF EXISTS badge_enabled,
    DROP COLUMN IF EXISTS call_ringing,
    DROP COLUMN IF EXISTS vibrate,
    DROP COLUMN IF EXISTS sound;

DROP TABLE IF EXISTS push_devices;
