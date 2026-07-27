CREATE TABLE channel_collaboration_settings (
    channel_id uuid PRIMARY KEY REFERENCES channels (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    room_mode text NOT NULL DEFAULT 'internal'
        CHECK (room_mode IN ('internal', 'public', 'webinar')),
    meeting_provider text NOT NULL DEFAULT 'jitsi'
        CHECK (meeting_provider IN ('jitsi', 'webrtc')),
    meeting_room_key text NOT NULL DEFAULT encode(gen_random_bytes(18), 'hex'),
    public_access_enabled boolean NOT NULL DEFAULT false,
    public_token_hash text,
    public_token_prefix text,
    password_hash text,
    lobby_enabled boolean NOT NULL DEFAULT true,
    chat_locked boolean NOT NULL DEFAULT false,
    guest_microphone_enabled boolean NOT NULL DEFAULT false,
    guest_camera_enabled boolean NOT NULL DEFAULT false,
    default_participant_role text NOT NULL DEFAULT 'member'
        CHECK (default_participant_role IN ('moderator', 'presenter', 'member', 'listener')),
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, meeting_room_key)
);

CREATE UNIQUE INDEX channel_collaboration_public_token_uidx
    ON channel_collaboration_settings (public_token_hash)
    WHERE public_access_enabled = true AND public_token_hash IS NOT NULL;
CREATE INDEX channel_collaboration_workspace_idx
    ON channel_collaboration_settings (workspace_id, room_mode);
CREATE TRIGGER trg_channel_collaboration_settings_updated_at
    BEFORE UPDATE ON channel_collaboration_settings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE channel_collaboration_roles (
    channel_id uuid NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role text NOT NULL DEFAULT 'member'
        CHECK (role IN ('moderator', 'presenter', 'member', 'listener')),
    assigned_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (channel_id, user_id),
    FOREIGN KEY (channel_id, user_id)
        REFERENCES channel_members (channel_id, user_id) ON DELETE CASCADE
);

CREATE TRIGGER trg_channel_collaboration_roles_updated_at
    BEFORE UPDATE ON channel_collaboration_roles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE channel_guest_requests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id uuid NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    display_name text NOT NULL,
    status text NOT NULL DEFAULT 'waiting'
        CHECK (status IN ('waiting', 'approved', 'rejected', 'expired')),
    access_token_hash text NOT NULL UNIQUE,
    reviewed_by uuid REFERENCES users (id) ON DELETE SET NULL,
    reviewed_at timestamptz,
    expires_at timestamptz NOT NULL DEFAULT (now() + interval '12 hours'),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX channel_guest_requests_channel_status_idx
    ON channel_guest_requests (channel_id, status, created_at);
CREATE TRIGGER trg_channel_guest_requests_updated_at
    BEFORE UPDATE ON channel_guest_requests
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE channel_collaboration_documents (
    channel_id uuid NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    kind text NOT NULL CHECK (kind IN ('notes', 'whiteboard')),
    content jsonb NOT NULL DEFAULT '{}'::jsonb,
    version bigint NOT NULL DEFAULT 1,
    updated_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (channel_id, kind)
);

CREATE TRIGGER trg_channel_collaboration_documents_updated_at
    BEFORE UPDATE ON channel_collaboration_documents
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE channel_tasks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    channel_id uuid NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    source_message_id uuid,
    title text NOT NULL,
    description text,
    status text NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'in_progress', 'done', 'cancelled')),
    assignee_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    due_at timestamptz,
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    completed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (workspace_id, source_message_id)
        REFERENCES messages (workspace_id, id) ON DELETE SET NULL
);

CREATE INDEX channel_tasks_channel_status_idx
    ON channel_tasks (channel_id, status, created_at DESC);
CREATE UNIQUE INDEX channel_tasks_message_uidx
    ON channel_tasks (channel_id, source_message_id)
    WHERE source_message_id IS NOT NULL AND status <> 'cancelled';
CREATE TRIGGER trg_channel_tasks_updated_at
    BEFORE UPDATE ON channel_tasks
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE channel_breakout_rooms (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id uuid NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    name text NOT NULL,
    room_key text NOT NULL DEFAULT encode(gen_random_bytes(12), 'hex'),
    assigned_user_ids jsonb NOT NULL DEFAULT '[]'::jsonb,
    status text NOT NULL DEFAULT 'open'
        CHECK (status IN ('open', 'closed')),
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    closed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (channel_id, room_key)
);

CREATE INDEX channel_breakout_rooms_channel_status_idx
    ON channel_breakout_rooms (channel_id, status, created_at);
CREATE TRIGGER trg_channel_breakout_rooms_updated_at
    BEFORE UPDATE ON channel_breakout_rooms
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
