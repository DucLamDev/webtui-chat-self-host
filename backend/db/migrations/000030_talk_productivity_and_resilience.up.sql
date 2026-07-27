-- Daily Talk productivity, resumable media, meeting lifecycle and optional
-- self-hosted intelligence. Heavy services remain opt-in; the core schema is
-- deliberately PostgreSQL-only.

ALTER TABLE notifications
    DROP CONSTRAINT notifications_type_check;
ALTER TABLE notifications
    ADD CONSTRAINT notifications_type_check
    CHECK (type IN ('mention', 'message', 'invite', 'system', 'webhook', 'bot',
                    'reminder', 'meeting', 'recording'));

ALTER TABLE notification_channel_preferences
    ADD COLUMN sensitive boolean NOT NULL DEFAULT false,
    ADD COLUMN important boolean NOT NULL DEFAULT false,
    ADD COLUMN compact boolean NOT NULL DEFAULT false,
    ADD COLUMN tags jsonb NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN archived_at timestamptz;

ALTER TABLE notification_channel_preferences
    ADD CONSTRAINT notification_channel_preferences_tags_array_check
    CHECK (jsonb_typeof(tags) = 'array');

CREATE INDEX notification_channel_preferences_user_important_idx
    ON notification_channel_preferences (workspace_id, user_id, updated_at DESC)
    WHERE important = true;
CREATE INDEX notification_channel_preferences_user_archived_idx
    ON notification_channel_preferences (workspace_id, user_id, archived_at DESC)
    WHERE archived_at IS NOT NULL;
CREATE INDEX notification_channel_preferences_tags_gin_idx
    ON notification_channel_preferences USING gin (tags);

ALTER TABLE message_pins
    ADD COLUMN expires_at timestamptz;

CREATE INDEX message_pins_active_expiry_idx
    ON message_pins (expires_at)
    WHERE expires_at IS NOT NULL;

CREATE TABLE scheduled_messages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    channel_id uuid NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    sender_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    parent_id uuid,
    kind text NOT NULL DEFAULT 'text'
        CHECK (kind IN ('text', 'file', 'event')),
    body text NOT NULL,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    mentioned_user_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
    client_message_id text,
    scheduled_for timestamptz NOT NULL,
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'sent', 'cancelled', 'failed')),
    sent_message_id uuid,
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id, parent_id)
        REFERENCES messages (workspace_id, id) ON DELETE SET NULL,
    FOREIGN KEY (workspace_id, sent_message_id)
        REFERENCES messages (workspace_id, id) ON DELETE SET NULL,
    CHECK (jsonb_typeof(mentioned_user_ids) = 'array')
);

CREATE UNIQUE INDEX scheduled_messages_client_id_uidx
    ON scheduled_messages (workspace_id, sender_id, client_message_id)
    WHERE client_message_id IS NOT NULL;
CREATE INDEX scheduled_messages_due_idx
    ON scheduled_messages (scheduled_for, created_at)
    WHERE status IN ('pending', 'failed');
CREATE INDEX scheduled_messages_sender_idx
    ON scheduled_messages (workspace_id, sender_id, status, scheduled_for);
CREATE TRIGGER trg_scheduled_messages_updated_at
    BEFORE UPDATE ON scheduled_messages
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE message_reminders (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    channel_id uuid NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    message_id uuid NOT NULL,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    remind_at timestamptz NOT NULL,
    note text,
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'fired', 'cancelled', 'failed')),
    notification_id uuid REFERENCES notifications (id) ON DELETE SET NULL,
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id, message_id)
        REFERENCES messages (workspace_id, id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX message_reminders_active_uidx
    ON message_reminders (workspace_id, message_id, user_id)
    WHERE status IN ('pending', 'processing', 'failed');
CREATE INDEX message_reminders_due_idx
    ON message_reminders (remind_at, created_at)
    WHERE status IN ('pending', 'failed');
CREATE INDEX message_reminders_user_idx
    ON message_reminders (workspace_id, user_id, status, remind_at);
CREATE TRIGGER trg_message_reminders_updated_at
    BEFORE UPDATE ON message_reminders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE message_threads (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    channel_id uuid NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    root_message_id uuid NOT NULL,
    title text NOT NULL DEFAULT '',
    description text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'resolved')),
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    resolved_by uuid REFERENCES users (id) ON DELETE SET NULL,
    resolved_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, root_message_id),
    FOREIGN KEY (workspace_id, root_message_id)
        REFERENCES messages (workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX message_threads_channel_updated_idx
    ON message_threads (workspace_id, channel_id, updated_at DESC);
CREATE TRIGGER trg_message_threads_updated_at
    BEFORE UPDATE ON message_threads
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE message_thread_subscriptions (
    workspace_id uuid NOT NULL,
    root_message_id uuid NOT NULL,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    subscribed boolean NOT NULL DEFAULT true,
    last_read_at timestamptz,
    last_read_message_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, root_message_id, user_id),
    FOREIGN KEY (workspace_id, root_message_id)
        REFERENCES message_threads (workspace_id, root_message_id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, last_read_message_id)
        REFERENCES messages (workspace_id, id) ON DELETE SET NULL
);

CREATE INDEX message_thread_subscriptions_user_idx
    ON message_thread_subscriptions (workspace_id, user_id, updated_at DESC)
    WHERE subscribed = true;
CREATE TRIGGER trg_message_thread_subscriptions_updated_at
    BEFORE UPDATE ON message_thread_subscriptions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE file_upload_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    owner_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    channel_id uuid REFERENCES channels (id) ON DELETE CASCADE,
    message_id uuid,
    original_name text NOT NULL,
    mime_type text NOT NULL,
    total_size bigint NOT NULL CHECK (total_size > 0),
    chunk_size integer NOT NULL CHECK (chunk_size BETWEEN 262144 AND 16777216),
    total_chunks integer NOT NULL CHECK (total_chunks BETWEEN 1 AND 10000),
    received_bytes bigint NOT NULL DEFAULT 0 CHECK (received_bytes >= 0),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL DEFAULT 'uploading'
        CHECK (status IN ('uploading', 'completing', 'completed', 'cancelled', 'expired', 'failed')),
    file_id uuid REFERENCES files (id) ON DELETE SET NULL,
    checksum_sha256 text,
    expires_at timestamptz NOT NULL DEFAULT (now() + interval '24 hours'),
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id, message_id)
        REFERENCES messages (workspace_id, id) ON DELETE SET NULL
);

CREATE INDEX file_upload_sessions_owner_idx
    ON file_upload_sessions (workspace_id, owner_id, status, updated_at DESC);
CREATE INDEX file_upload_sessions_expiry_idx
    ON file_upload_sessions (expires_at)
    WHERE status = 'uploading';
CREATE TRIGGER trg_file_upload_sessions_updated_at
    BEFORE UPDATE ON file_upload_sessions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE file_upload_parts (
    upload_id uuid NOT NULL REFERENCES file_upload_sessions (id) ON DELETE CASCADE,
    part_number integer NOT NULL CHECK (part_number >= 0),
    object_key text NOT NULL,
    byte_size integer NOT NULL CHECK (byte_size > 0),
    checksum_sha256 text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (upload_id, part_number),
    UNIQUE (object_key)
);

CREATE TRIGGER trg_file_upload_parts_updated_at
    BEFORE UPDATE ON file_upload_parts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE channel_meetings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    channel_id uuid NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    starts_at timestamptz NOT NULL,
    ends_at timestamptz,
    lobby_opens_at timestamptz,
    status text NOT NULL DEFAULT 'scheduled'
        CHECK (status IN ('scheduled', 'active', 'ended', 'cancelled')),
    room_policy text NOT NULL DEFAULT 'keep'
        CHECK (room_policy IN ('keep', 'archive', 'delete')),
    cleanup_after timestamptz,
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    started_at timestamptz,
    ended_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX channel_meetings_channel_idx
    ON channel_meetings (workspace_id, channel_id, starts_at DESC);
CREATE INDEX channel_meetings_upcoming_idx
    ON channel_meetings (workspace_id, starts_at)
    WHERE status = 'scheduled';
CREATE INDEX channel_meetings_cleanup_idx
    ON channel_meetings (cleanup_after)
    WHERE status = 'ended' AND room_policy <> 'keep';
CREATE TRIGGER trg_channel_meetings_updated_at
    BEFORE UPDATE ON channel_meetings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE channel_voice_rooms (
    channel_id uuid PRIMARY KEY REFERENCES channels (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'inactive'
        CHECK (status IN ('inactive', 'active')),
    started_by uuid REFERENCES users (id) ON DELETE SET NULL,
    started_at timestamptz,
    ended_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX channel_voice_rooms_active_idx
    ON channel_voice_rooms (workspace_id, started_at DESC)
    WHERE status = 'active';
CREATE TRIGGER trg_channel_voice_rooms_updated_at
    BEFORE UPDATE ON channel_voice_rooms
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE channel_breakout_rooms
    DROP CONSTRAINT channel_breakout_rooms_status_check;
ALTER TABLE channel_breakout_rooms
    ALTER COLUMN status SET DEFAULT 'prepared';
ALTER TABLE channel_breakout_rooms
    ADD CONSTRAINT channel_breakout_rooms_status_check
    CHECK (status IN ('prepared', 'active', 'closed'));
ALTER TABLE channel_breakout_rooms
    ADD COLUMN assignment_mode text NOT NULL DEFAULT 'manual'
        CHECK (assignment_mode IN ('manual', 'automatic', 'self_select')),
    ADD COLUMN allow_self_select boolean NOT NULL DEFAULT false,
    ADD COLUMN started_at timestamptz,
    ADD COLUMN sequence integer NOT NULL DEFAULT 0;

CREATE TABLE channel_breakout_broadcasts (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id uuid NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    body text NOT NULL,
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX channel_breakout_broadcasts_channel_idx
    ON channel_breakout_broadcasts (channel_id, created_at DESC);

CREATE TABLE channel_recording_policies (
    channel_id uuid PRIMARY KEY REFERENCES channels (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    enabled boolean NOT NULL DEFAULT false,
    consent_required boolean NOT NULL DEFAULT true,
    retention_days integer NOT NULL DEFAULT 30 CHECK (retention_days BETWEEN 1 AND 3650),
    transcription_enabled boolean NOT NULL DEFAULT false,
    summary_enabled boolean NOT NULL DEFAULT false,
    provider text NOT NULL DEFAULT 'jibri',
    updated_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_channel_recording_policies_updated_at
    BEFORE UPDATE ON channel_recording_policies
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE channel_recordings (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    channel_id uuid NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    meeting_id uuid REFERENCES channel_meetings (id) ON DELETE SET NULL,
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'recording', 'processing', 'ready', 'failed', 'deleted')),
    provider text NOT NULL DEFAULT 'jibri',
    provider_recording_id text,
    participant_user_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
    storage_key text,
    mime_type text,
    byte_size bigint,
    checksum_sha256 text,
    started_by uuid REFERENCES users (id) ON DELETE SET NULL,
    started_at timestamptz,
    ended_at timestamptz,
    expires_at timestamptz,
    transcript_status text NOT NULL DEFAULT 'disabled'
        CHECK (transcript_status IN ('disabled', 'pending', 'processing', 'ready', 'failed')),
    transcript jsonb NOT NULL DEFAULT '{}'::jsonb,
    summary_status text NOT NULL DEFAULT 'disabled'
        CHECK (summary_status IN ('disabled', 'pending', 'processing', 'ready', 'failed')),
    summary jsonb NOT NULL DEFAULT '{}'::jsonb,
    error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX channel_recordings_channel_idx
    ON channel_recordings (workspace_id, channel_id, created_at DESC);
CREATE INDEX channel_recordings_processing_idx
    ON channel_recordings (status, updated_at)
    WHERE status IN ('pending', 'processing');
CREATE UNIQUE INDEX channel_recordings_active_uidx
    ON channel_recordings (channel_id)
    WHERE status IN ('pending', 'recording');
CREATE INDEX channel_recordings_expiry_idx
    ON channel_recordings (expires_at)
    WHERE status = 'ready' AND expires_at IS NOT NULL;
CREATE TRIGGER trg_channel_recordings_updated_at
    BEFORE UPDATE ON channel_recordings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE channel_recording_consents (
    recording_id uuid NOT NULL REFERENCES channel_recordings (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    consented boolean NOT NULL,
    consented_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (recording_id, user_id)
);

CREATE TABLE workspace_talk_integrations (
    workspace_id uuid PRIMARY KEY REFERENCES workspaces (id) ON DELETE CASCADE,
    ai_enabled boolean NOT NULL DEFAULT false,
    ai_provider text NOT NULL DEFAULT 'ollama',
    transcription_provider text NOT NULL DEFAULT 'faster_whisper',
    federation_enabled boolean NOT NULL DEFAULT false,
    e2ee_calls_enabled boolean NOT NULL DEFAULT false,
    sip_enabled boolean NOT NULL DEFAULT false,
    bridge_enabled boolean NOT NULL DEFAULT false,
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    updated_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_workspace_talk_integrations_updated_at
    BEFORE UPDATE ON workspace_talk_integrations
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE federated_conversation_invites (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    channel_id uuid NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    remote_server text NOT NULL,
    remote_user text NOT NULL,
    direction text NOT NULL CHECK (direction IN ('outbound', 'inbound')),
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'declined', 'revoked', 'failed')),
    protocol text NOT NULL DEFAULT 'open_cloud_mesh',
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    responded_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX federated_conversation_invites_workspace_idx
    ON federated_conversation_invites (workspace_id, status, created_at DESC);
CREATE TRIGGER trg_federated_conversation_invites_updated_at
    BEFORE UPDATE ON federated_conversation_invites
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
