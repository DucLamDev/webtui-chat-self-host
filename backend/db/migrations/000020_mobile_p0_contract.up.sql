CREATE TABLE push_devices (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    workspace_id uuid REFERENCES workspaces (id) ON DELETE CASCADE,
    device_id text NOT NULL,
    platform text NOT NULL CHECK (platform IN ('android', 'ios', 'desktop', 'web')),
    push_provider text NOT NULL DEFAULT 'fcm' CHECK (push_provider IN ('fcm', 'apns', 'none')),
    push_token text,
    notification_permission text NOT NULL DEFAULT 'unknown' CHECK (notification_permission IN ('granted', 'denied', 'provisional', 'unknown')),
    app_version text,
    build_number text,
    release_channel text,
    locale text,
    timezone text,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, device_id)
);

CREATE INDEX push_devices_user_status_idx ON push_devices (user_id, status, updated_at DESC);
CREATE INDEX push_devices_workspace_status_idx ON push_devices (workspace_id, status, updated_at DESC);
CREATE TRIGGER trg_push_devices_updated_at BEFORE UPDATE ON push_devices FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE notification_preferences
    ADD COLUMN IF NOT EXISTS sound boolean NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS vibrate boolean NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS call_ringing boolean NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS badge_enabled boolean NOT NULL DEFAULT true;

CREATE TABLE notification_channel_preferences (
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    channel_id uuid NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    mode text NOT NULL DEFAULT 'all' CHECK (mode IN ('all', 'mentions', 'muted')),
    muted_until timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, workspace_id, channel_id)
);

CREATE INDEX notification_channel_preferences_workspace_idx
ON notification_channel_preferences (workspace_id, channel_id, updated_at DESC);
CREATE TRIGGER trg_notification_channel_preferences_updated_at
BEFORE UPDATE ON notification_channel_preferences
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE mobile_sync_cursors (
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    device_id text NOT NULL,
    cursor_event_id uuid,
    cursor_created_at timestamptz,
    acked_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, workspace_id, device_id),
    FOREIGN KEY (cursor_event_id) REFERENCES outbox_events (id) ON DELETE SET NULL
);

CREATE INDEX mobile_sync_cursors_workspace_idx ON mobile_sync_cursors (workspace_id, acked_at DESC);
CREATE TRIGGER trg_mobile_sync_cursors_updated_at BEFORE UPDATE ON mobile_sync_cursors FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE call_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    channel_id uuid NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    initiator_user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    target_user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    client_call_id text,
    mode text NOT NULL CHECK (mode IN ('audio', 'video')),
    status text NOT NULL DEFAULT 'ringing' CHECK (status IN ('ringing', 'accepted', 'rejected', 'cancelled', 'ended', 'missed')),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    started_at timestamptz,
    ended_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (initiator_user_id <> target_user_id)
);

CREATE UNIQUE INDEX call_sessions_client_call_uidx
ON call_sessions (workspace_id, initiator_user_id, client_call_id)
WHERE client_call_id IS NOT NULL AND length(trim(client_call_id)) > 0;
CREATE INDEX call_sessions_channel_created_idx ON call_sessions (workspace_id, channel_id, created_at DESC);
CREATE INDEX call_sessions_target_status_idx ON call_sessions (target_user_id, status, created_at DESC);
CREATE TRIGGER trg_call_sessions_updated_at BEFORE UPDATE ON call_sessions FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE call_signals (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    call_id uuid NOT NULL REFERENCES call_sessions (id) ON DELETE CASCADE,
    sender_user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    signal_type text NOT NULL CHECK (signal_type IN ('offer', 'answer', 'ice_candidate', 'ringing', 'accepted', 'rejected', 'cancelled', 'ended', 'missed')),
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX call_signals_call_created_idx ON call_signals (call_id, created_at ASC);

CREATE TABLE bot_ai_configs (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    bot_id uuid NOT NULL REFERENCES bots (id) ON DELETE CASCADE,
    provider text NOT NULL,
    model text NOT NULL,
    secret_ref text,
    settings jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    updated_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, bot_id)
);

CREATE TRIGGER trg_bot_ai_configs_updated_at BEFORE UPDATE ON bot_ai_configs FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE bot_flows (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    bot_id uuid NOT NULL REFERENCES bots (id) ON DELETE CASCADE,
    version integer NOT NULL DEFAULT 1 CHECK (version > 0),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published', 'archived')),
    name text NOT NULL,
    prompt text NOT NULL,
    trigger_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    tool_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    knowledge_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    updated_by uuid REFERENCES users (id) ON DELETE SET NULL,
    published_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, bot_id, version)
);

CREATE INDEX bot_flows_bot_status_idx ON bot_flows (workspace_id, bot_id, status, version DESC);
CREATE UNIQUE INDEX bot_flows_single_published_uidx
ON bot_flows (workspace_id, bot_id)
WHERE status = 'published';
CREATE TRIGGER trg_bot_flows_updated_at BEFORE UPDATE ON bot_flows FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE bot_flow_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    bot_id uuid NOT NULL REFERENCES bots (id) ON DELETE CASCADE,
    flow_id uuid NOT NULL REFERENCES bot_flows (id) ON DELETE CASCADE,
    input jsonb NOT NULL DEFAULT '{}'::jsonb,
    transcript jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL DEFAULT 'success' CHECK (status IN ('success', 'failed')),
    error text,
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX bot_flow_runs_flow_created_idx ON bot_flow_runs (flow_id, created_at DESC);
