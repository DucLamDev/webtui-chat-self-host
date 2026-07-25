-- Initial PostgreSQL schema for WebTui Chat.

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TABLE users (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email citext NOT NULL,
    username citext NOT NULL,
    display_name text NOT NULL,
    password_hash text NOT NULL,
    avatar_url text,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'invited', 'locked', 'disabled')),
    locale text NOT NULL DEFAULT 'vi-VN',
    timezone text NOT NULL DEFAULT 'Asia/Ho_Chi_Minh',
    email_verified_at timestamptz,
    last_seen_at timestamptz,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX users_email_active_uidx ON users (email) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX users_username_active_uidx ON users (username) WHERE deleted_at IS NULL;
CREATE INDEX users_status_idx ON users (status) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE user_sessions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    refresh_token_hash text NOT NULL UNIQUE,
    device_name text,
    ip_address inet,
    user_agent text,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX user_sessions_user_id_idx ON user_sessions (user_id, created_at DESC);
CREATE INDEX user_sessions_active_idx ON user_sessions (user_id, expires_at) WHERE revoked_at IS NULL;
CREATE TRIGGER trg_user_sessions_updated_at BEFORE UPDATE ON user_sessions FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE workspaces (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug citext NOT NULL,
    name text NOT NULL,
    description text,
    owner_id uuid REFERENCES users (id) ON DELETE SET NULL,
    plan text NOT NULL DEFAULT 'self_hosted' CHECK (plan IN ('self_hosted', 'free', 'team', 'enterprise')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'archived')),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX workspaces_slug_active_uidx ON workspaces (slug) WHERE deleted_at IS NULL;
CREATE INDEX workspaces_owner_id_idx ON workspaces (owner_id);
CREATE TRIGGER trg_workspaces_updated_at BEFORE UPDATE ON workspaces FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE workspace_settings (
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    key text NOT NULL,
    value jsonb NOT NULL,
    value_type text NOT NULL DEFAULT 'json' CHECK (value_type IN ('string', 'number', 'boolean', 'json')),
    description text,
    updated_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, key)
);

CREATE INDEX workspace_settings_key_idx ON workspace_settings (key);
CREATE TRIGGER trg_workspace_settings_updated_at BEFORE UPDATE ON workspace_settings FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE workspace_members (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'invited', 'disabled', 'left')),
    title text,
    joined_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, user_id)
);

CREATE INDEX workspace_members_user_id_idx ON workspace_members (user_id, status);
CREATE INDEX workspace_members_workspace_status_idx ON workspace_members (workspace_id, status);
CREATE TRIGGER trg_workspace_members_updated_at BEFORE UPDATE ON workspace_members FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE permissions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code citext NOT NULL UNIQUE,
    module text NOT NULL,
    action text NOT NULL,
    name text NOT NULL,
    description text,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX permissions_module_action_idx ON permissions (module, action);

CREATE TABLE roles (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid REFERENCES workspaces (id) ON DELETE CASCADE,
    code citext NOT NULL,
    name text NOT NULL,
    description text,
    is_system boolean NOT NULL DEFAULT false,
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX roles_global_code_uidx ON roles (code) WHERE workspace_id IS NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX roles_workspace_code_uidx ON roles (workspace_id, code) WHERE workspace_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX roles_workspace_idx ON roles (workspace_id) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_roles_updated_at BEFORE UPDATE ON roles FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE role_permissions (
    role_id uuid NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    permission_id uuid NOT NULL REFERENCES permissions (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (role_id, permission_id)
);

CREATE INDEX role_permissions_permission_id_idx ON role_permissions (permission_id);

CREATE TABLE workspace_member_roles (
    workspace_id uuid NOT NULL,
    user_id uuid NOT NULL,
    role_id uuid NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    assigned_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, user_id, role_id),
    FOREIGN KEY (workspace_id, user_id) REFERENCES workspace_members (workspace_id, user_id) ON DELETE CASCADE
);

CREATE INDEX workspace_member_roles_role_id_idx ON workspace_member_roles (role_id);

CREATE TABLE departments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    parent_id uuid REFERENCES departments (id) ON DELETE SET NULL,
    slug citext NOT NULL,
    name text NOT NULL,
    description text,
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX departments_workspace_slug_active_uidx ON departments (workspace_id, slug) WHERE deleted_at IS NULL;
CREATE INDEX departments_workspace_parent_idx ON departments (workspace_id, parent_id);
CREATE TRIGGER trg_departments_updated_at BEFORE UPDATE ON departments FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE department_members (
    department_id uuid NOT NULL REFERENCES departments (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role text NOT NULL DEFAULT 'member' CHECK (role IN ('lead', 'member')),
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (department_id, user_id)
);

CREATE INDEX department_members_user_id_idx ON department_members (user_id);

CREATE TABLE workspace_invites (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    email citext NOT NULL,
    role_id uuid REFERENCES roles (id) ON DELETE SET NULL,
    token_hash text NOT NULL UNIQUE,
    invited_by uuid REFERENCES users (id) ON DELETE SET NULL,
    accepted_by uuid REFERENCES users (id) ON DELETE SET NULL,
    expires_at timestamptz NOT NULL,
    accepted_at timestamptz,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX workspace_invites_workspace_email_idx ON workspace_invites (workspace_id, email);
CREATE INDEX workspace_invites_active_idx ON workspace_invites (workspace_id, expires_at) WHERE accepted_at IS NULL AND revoked_at IS NULL;

CREATE TABLE files (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid REFERENCES workspaces (id) ON DELETE CASCADE,
    owner_id uuid REFERENCES users (id) ON DELETE SET NULL,
    storage_provider text NOT NULL DEFAULT 'local' CHECK (storage_provider IN ('local', 'minio', 's3')),
    bucket text,
    object_key text NOT NULL,
    original_name text NOT NULL,
    mime_type text NOT NULL,
    byte_size bigint NOT NULL CHECK (byte_size >= 0),
    checksum_sha256 text,
    status text NOT NULL DEFAULT 'uploaded' CHECK (status IN ('uploading', 'uploaded', 'processing', 'ready', 'failed', 'deleted')),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX files_storage_object_uidx ON files (storage_provider, COALESCE(bucket, ''), object_key);
CREATE INDEX files_workspace_created_idx ON files (workspace_id, created_at DESC);
CREATE INDEX files_owner_created_idx ON files (owner_id, created_at DESC);
CREATE INDEX files_metadata_gin_idx ON files USING gin (metadata);
CREATE TRIGGER trg_files_updated_at BEFORE UPDATE ON files FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE file_versions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    file_id uuid NOT NULL REFERENCES files (id) ON DELETE CASCADE,
    version_number integer NOT NULL CHECK (version_number > 0),
    storage_provider text NOT NULL DEFAULT 'local' CHECK (storage_provider IN ('local', 'minio', 's3')),
    bucket text,
    object_key text NOT NULL,
    mime_type text NOT NULL,
    byte_size bigint NOT NULL CHECK (byte_size >= 0),
    checksum_sha256 text,
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (file_id, version_number)
);

CREATE UNIQUE INDEX file_versions_storage_object_uidx ON file_versions (storage_provider, COALESCE(bucket, ''), object_key);
CREATE INDEX file_versions_file_created_idx ON file_versions (file_id, created_at DESC);

CREATE TABLE channels (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    department_id uuid REFERENCES departments (id) ON DELETE SET NULL,
    slug citext,
    name text NOT NULL,
    description text,
    type text NOT NULL DEFAULT 'public' CHECK (type IN ('public', 'private', 'direct', 'group')),
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived', 'deleted')),
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    settings jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz,
    deleted_at timestamptz
);

CREATE UNIQUE INDEX channels_workspace_slug_active_uidx ON channels (workspace_id, slug) WHERE slug IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX channels_workspace_type_idx ON channels (workspace_id, type, status);
CREATE INDEX channels_department_idx ON channels (department_id) WHERE department_id IS NOT NULL;
CREATE TRIGGER trg_channels_updated_at BEFORE UPDATE ON channels FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE direct_conversations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    channel_id uuid NOT NULL UNIQUE REFERENCES channels (id) ON DELETE CASCADE,
    participant_key text NOT NULL,
    conversation_type text NOT NULL DEFAULT 'one_to_one' CHECK (conversation_type IN ('one_to_one', 'group')),
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    last_message_id uuid,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    archived_at timestamptz
);

CREATE UNIQUE INDEX direct_conversations_participant_key_uidx ON direct_conversations (workspace_id, participant_key) WHERE archived_at IS NULL;
CREATE INDEX direct_conversations_workspace_updated_idx ON direct_conversations (workspace_id, updated_at DESC);
CREATE TRIGGER trg_direct_conversations_updated_at BEFORE UPDATE ON direct_conversations FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE direct_conversation_members (
    direct_conversation_id uuid NOT NULL REFERENCES direct_conversations (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (direct_conversation_id, user_id)
);

CREATE INDEX direct_conversation_members_user_id_idx ON direct_conversation_members (user_id);

CREATE TABLE channel_members (
    channel_id uuid NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'invited', 'muted', 'left', 'removed')),
    last_read_at timestamptz,
    last_read_message_id uuid,
    joined_at timestamptz NOT NULL DEFAULT now(),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (channel_id, user_id)
);

CREATE INDEX channel_members_user_status_idx ON channel_members (user_id, status);
CREATE INDEX channel_members_channel_status_idx ON channel_members (channel_id, status);
CREATE TRIGGER trg_channel_members_updated_at BEFORE UPDATE ON channel_members FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE channel_member_roles (
    channel_id uuid NOT NULL,
    user_id uuid NOT NULL,
    role_id uuid NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    assigned_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (channel_id, user_id, role_id),
    FOREIGN KEY (channel_id, user_id) REFERENCES channel_members (channel_id, user_id) ON DELETE CASCADE
);

CREATE INDEX channel_member_roles_role_id_idx ON channel_member_roles (role_id);

CREATE TABLE user_presence (
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    workspace_id uuid REFERENCES workspaces (id) ON DELETE CASCADE,
    device_id text NOT NULL,
    socket_id text NOT NULL,
    node_id text NOT NULL,
    status text NOT NULL DEFAULT 'online' CHECK (status IN ('online', 'away', 'offline')),
    last_heartbeat_at timestamptz NOT NULL DEFAULT now(),
    connected_at timestamptz NOT NULL DEFAULT now(),
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (user_id, device_id)
);

CREATE UNIQUE INDEX user_presence_socket_uidx ON user_presence (socket_id);
CREATE INDEX user_presence_workspace_status_idx ON user_presence (workspace_id, status, last_heartbeat_at DESC);

CREATE TABLE messages (
    id uuid NOT NULL DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    channel_id uuid NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    sender_id uuid REFERENCES users (id) ON DELETE SET NULL,
    parent_id uuid,
    thread_root_id uuid,
    kind text NOT NULL DEFAULT 'text' CHECK (kind IN ('text', 'file', 'system', 'bot', 'event')),
    body text NOT NULL DEFAULT '',
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    search_vector tsvector GENERATED ALWAYS AS (to_tsvector('simple', coalesce(body, ''))) STORED,
    edited_at timestamptz,
    deleted_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, id),
    FOREIGN KEY (workspace_id, parent_id) REFERENCES messages (workspace_id, id),
    FOREIGN KEY (workspace_id, thread_root_id) REFERENCES messages (workspace_id, id),
    CHECK (kind <> 'text' OR length(trim(body)) > 0),
    CHECK (thread_root_id IS NULL OR thread_root_id <> id)
) PARTITION BY HASH (workspace_id);

CREATE TABLE messages_p00 PARTITION OF messages FOR VALUES WITH (MODULUS 16, REMAINDER 0);
CREATE TABLE messages_p01 PARTITION OF messages FOR VALUES WITH (MODULUS 16, REMAINDER 1);
CREATE TABLE messages_p02 PARTITION OF messages FOR VALUES WITH (MODULUS 16, REMAINDER 2);
CREATE TABLE messages_p03 PARTITION OF messages FOR VALUES WITH (MODULUS 16, REMAINDER 3);
CREATE TABLE messages_p04 PARTITION OF messages FOR VALUES WITH (MODULUS 16, REMAINDER 4);
CREATE TABLE messages_p05 PARTITION OF messages FOR VALUES WITH (MODULUS 16, REMAINDER 5);
CREATE TABLE messages_p06 PARTITION OF messages FOR VALUES WITH (MODULUS 16, REMAINDER 6);
CREATE TABLE messages_p07 PARTITION OF messages FOR VALUES WITH (MODULUS 16, REMAINDER 7);
CREATE TABLE messages_p08 PARTITION OF messages FOR VALUES WITH (MODULUS 16, REMAINDER 8);
CREATE TABLE messages_p09 PARTITION OF messages FOR VALUES WITH (MODULUS 16, REMAINDER 9);
CREATE TABLE messages_p10 PARTITION OF messages FOR VALUES WITH (MODULUS 16, REMAINDER 10);
CREATE TABLE messages_p11 PARTITION OF messages FOR VALUES WITH (MODULUS 16, REMAINDER 11);
CREATE TABLE messages_p12 PARTITION OF messages FOR VALUES WITH (MODULUS 16, REMAINDER 12);
CREATE TABLE messages_p13 PARTITION OF messages FOR VALUES WITH (MODULUS 16, REMAINDER 13);
CREATE TABLE messages_p14 PARTITION OF messages FOR VALUES WITH (MODULUS 16, REMAINDER 14);
CREATE TABLE messages_p15 PARTITION OF messages FOR VALUES WITH (MODULUS 16, REMAINDER 15);

CREATE INDEX messages_channel_timeline_idx ON messages (workspace_id, channel_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX messages_workspace_created_idx ON messages (workspace_id, created_at DESC);
CREATE INDEX messages_sender_created_idx ON messages (workspace_id, sender_id, created_at DESC);
CREATE INDEX messages_parent_idx ON messages (workspace_id, parent_id) WHERE parent_id IS NOT NULL;
CREATE INDEX messages_thread_root_idx ON messages (workspace_id, thread_root_id, created_at ASC) WHERE thread_root_id IS NOT NULL;
CREATE INDEX messages_metadata_gin_idx ON messages USING gin (metadata);
CREATE INDEX messages_search_vector_gin_idx ON messages USING gin (search_vector);
CREATE TRIGGER trg_messages_updated_at BEFORE UPDATE ON messages FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE message_reactions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL,
    message_id uuid NOT NULL,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    emoji text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, message_id, user_id, emoji),
    FOREIGN KEY (workspace_id, message_id) REFERENCES messages (workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX message_reactions_user_id_idx ON message_reactions (user_id);

CREATE TABLE message_mentions (
    workspace_id uuid NOT NULL,
    message_id uuid NOT NULL,
    mentioned_user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, message_id, mentioned_user_id),
    FOREIGN KEY (workspace_id, message_id) REFERENCES messages (workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX message_mentions_user_id_idx ON message_mentions (mentioned_user_id, created_at DESC);

CREATE TABLE message_attachments (
    workspace_id uuid NOT NULL,
    message_id uuid NOT NULL,
    file_id uuid NOT NULL REFERENCES files (id) ON DELETE RESTRICT,
    sort_order integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, message_id, file_id),
    FOREIGN KEY (workspace_id, message_id) REFERENCES messages (workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX message_attachments_file_id_idx ON message_attachments (file_id);

CREATE TABLE message_reads (
    workspace_id uuid NOT NULL,
    message_id uuid NOT NULL,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    receipt_type text NOT NULL DEFAULT 'explicit' CHECK (receipt_type IN ('explicit', 'compliance', 'debug')),
    read_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, message_id, user_id, receipt_type),
    FOREIGN KEY (workspace_id, message_id) REFERENCES messages (workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX message_reads_user_read_at_idx ON message_reads (user_id, read_at DESC);

CREATE TABLE search_documents (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    source_type text NOT NULL CHECK (source_type IN ('message', 'file', 'channel', 'workspace', 'bot')),
    source_id uuid NOT NULL,
    title text NOT NULL DEFAULT '',
    body text NOT NULL DEFAULT '',
    search_vector tsvector GENERATED ALWAYS AS (to_tsvector('simple', coalesce(title, '') || ' ' || coalesce(body, ''))) STORED,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, source_type, source_id)
);

CREATE INDEX search_documents_vector_gin_idx ON search_documents USING gin (search_vector);
CREATE INDEX search_documents_workspace_type_idx ON search_documents (workspace_id, source_type);
CREATE TRIGGER trg_search_documents_updated_at BEFORE UPDATE ON search_documents FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE notifications (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    workspace_id uuid REFERENCES workspaces (id) ON DELETE CASCADE,
    channel_id uuid REFERENCES channels (id) ON DELETE CASCADE,
    message_id uuid,
    type text NOT NULL CHECK (type IN ('mention', 'message', 'invite', 'system', 'webhook', 'bot')),
    title text NOT NULL,
    body text NOT NULL DEFAULT '',
    data jsonb NOT NULL DEFAULT '{}'::jsonb,
    read_at timestamptz,
    delivered_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (message_id IS NULL OR workspace_id IS NOT NULL),
    FOREIGN KEY (workspace_id, message_id) REFERENCES messages (workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX notifications_user_unread_idx ON notifications (user_id, created_at DESC) WHERE read_at IS NULL;
CREATE INDEX notifications_workspace_created_idx ON notifications (workspace_id, created_at DESC);

CREATE TABLE notification_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    notification_id uuid REFERENCES notifications (id) ON DELETE CASCADE,
    workspace_id uuid REFERENCES workspaces (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    channel text NOT NULL CHECK (channel IN ('desktop', 'push', 'email', 'webhook', 'sms')),
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'sent', 'failed', 'dead')),
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    next_attempt_at timestamptz,
    sent_at timestamptz,
    error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX notification_jobs_pending_idx ON notification_jobs (next_attempt_at, created_at) WHERE status IN ('pending', 'failed');
CREATE INDEX notification_jobs_user_channel_idx ON notification_jobs (user_id, channel, created_at DESC);
CREATE TRIGGER trg_notification_jobs_updated_at BEFORE UPDATE ON notification_jobs FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE bots (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    slug citext NOT NULL,
    name text NOT NULL,
    description text,
    avatar_url text,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    settings jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE UNIQUE INDEX bots_workspace_slug_active_uidx ON bots (workspace_id, slug) WHERE deleted_at IS NULL;
CREATE TRIGGER trg_bots_updated_at BEFORE UPDATE ON bots FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE bot_installations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    bot_id uuid NOT NULL REFERENCES bots (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    channel_id uuid REFERENCES channels (id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX bot_installations_workspace_global_uidx ON bot_installations (bot_id, workspace_id) WHERE channel_id IS NULL;
CREATE UNIQUE INDEX bot_installations_workspace_channel_uidx ON bot_installations (bot_id, workspace_id, channel_id) WHERE channel_id IS NOT NULL;
CREATE INDEX bot_installations_workspace_idx ON bot_installations (workspace_id, status);
CREATE TRIGGER trg_bot_installations_updated_at BEFORE UPDATE ON bot_installations FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE api_tokens (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    owner_id uuid REFERENCES users (id) ON DELETE SET NULL,
    name text NOT NULL,
    token_hash text NOT NULL UNIQUE,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked', 'expired')),
    last_used_at timestamptz,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz
);

CREATE INDEX api_tokens_workspace_status_idx ON api_tokens (workspace_id, status);
CREATE TRIGGER trg_api_tokens_updated_at BEFORE UPDATE ON api_tokens FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE api_scopes (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    code citext NOT NULL UNIQUE,
    name text NOT NULL,
    description text,
    module text NOT NULL,
    action text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX api_scopes_module_action_idx ON api_scopes (module, action);

CREATE TABLE api_token_scopes (
    api_token_id uuid NOT NULL REFERENCES api_tokens (id) ON DELETE CASCADE,
    api_scope_id uuid NOT NULL REFERENCES api_scopes (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (api_token_id, api_scope_id)
);

CREATE INDEX api_token_scopes_scope_id_idx ON api_token_scopes (api_scope_id);

CREATE TABLE incoming_webhooks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    channel_id uuid REFERENCES channels (id) ON DELETE SET NULL,
    name text NOT NULL,
    secret_hash text NOT NULL,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    allowed_ips cidr[],
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz
);

CREATE INDEX incoming_webhooks_workspace_status_idx ON incoming_webhooks (workspace_id, status);
CREATE TRIGGER trg_incoming_webhooks_updated_at BEFORE UPDATE ON incoming_webhooks FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE outgoing_webhooks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    name text NOT NULL,
    target_url text NOT NULL,
    secret_hash text NOT NULL,
    event_types text[] NOT NULL DEFAULT '{}',
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX outgoing_webhooks_workspace_status_idx ON outgoing_webhooks (workspace_id, status);
CREATE TRIGGER trg_outgoing_webhooks_updated_at BEFORE UPDATE ON outgoing_webhooks FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE webhook_deliveries (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    outgoing_webhook_id uuid NOT NULL REFERENCES outgoing_webhooks (id) ON DELETE CASCADE,
    event_id uuid,
    event_type text NOT NULL,
    request_body jsonb NOT NULL DEFAULT '{}'::jsonb,
    response_status integer,
    response_body text,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'delivering', 'success', 'failed', 'dead')),
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    next_attempt_at timestamptz,
    delivered_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX webhook_deliveries_pending_idx ON webhook_deliveries (next_attempt_at, created_at) WHERE status IN ('pending', 'failed');
CREATE INDEX webhook_deliveries_webhook_created_idx ON webhook_deliveries (outgoing_webhook_id, created_at DESC);
CREATE TRIGGER trg_webhook_deliveries_updated_at BEFORE UPDATE ON webhook_deliveries FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE cron_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL UNIQUE,
    description text,
    schedule text NOT NULL,
    runner text NOT NULL DEFAULT 'worker',
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'disabled')),
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    last_run_at timestamptz,
    next_run_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX cron_jobs_due_idx ON cron_jobs (next_run_at) WHERE status = 'active';
CREATE TRIGGER trg_cron_jobs_updated_at BEFORE UPDATE ON cron_jobs FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE cron_job_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    cron_job_id uuid NOT NULL REFERENCES cron_jobs (id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'success', 'failed', 'cancelled')),
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    log text,
    error text
);

CREATE INDEX cron_job_runs_job_started_idx ON cron_job_runs (cron_job_id, started_at DESC);

CREATE TABLE outbox_events (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type text NOT NULL,
    aggregate_id uuid NOT NULL,
    event_type text NOT NULL,
    event_version integer NOT NULL DEFAULT 1 CHECK (event_version > 0),
    payload jsonb NOT NULL DEFAULT '{}'::jsonb,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'published', 'failed', 'dead')),
    retry_count integer NOT NULL DEFAULT 0 CHECK (retry_count >= 0),
    next_attempt_at timestamptz,
    published_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX outbox_events_pending_idx ON outbox_events (created_at) WHERE status = 'pending';
CREATE INDEX outbox_events_retry_idx ON outbox_events (next_attempt_at) WHERE status = 'failed';
CREATE INDEX outbox_events_aggregate_idx ON outbox_events (aggregate_type, aggregate_id, created_at DESC);
CREATE INDEX outbox_events_payload_gin_idx ON outbox_events USING gin (payload);
CREATE TRIGGER trg_outbox_events_updated_at BEFORE UPDATE ON outbox_events FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE audit_logs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid REFERENCES workspaces (id) ON DELETE SET NULL,
    actor_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    action text NOT NULL,
    entity_type text NOT NULL,
    entity_id uuid,
    ip_address inet,
    user_agent text,
    before_data jsonb,
    after_data jsonb,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX audit_logs_workspace_created_idx ON audit_logs (workspace_id, created_at DESC);
CREATE INDEX audit_logs_actor_created_idx ON audit_logs (actor_user_id, created_at DESC);
CREATE INDEX audit_logs_entity_idx ON audit_logs (entity_type, entity_id);
CREATE INDEX audit_logs_metadata_gin_idx ON audit_logs USING gin (metadata);
CREATE INDEX audit_logs_before_data_gin_idx ON audit_logs USING gin (before_data);
CREATE INDEX audit_logs_after_data_gin_idx ON audit_logs USING gin (after_data);

CREATE TABLE backup_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid REFERENCES workspaces (id) ON DELETE SET NULL,
    name text NOT NULL,
    target text NOT NULL DEFAULT 'local' CHECK (target IN ('local', 'minio', 's3', 'remote')),
    schedule text,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'paused', 'disabled')),
    config jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX backup_jobs_status_idx ON backup_jobs (status);
CREATE TRIGGER trg_backup_jobs_updated_at BEFORE UPDATE ON backup_jobs FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE backup_runs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    backup_job_id uuid REFERENCES backup_jobs (id) ON DELETE SET NULL,
    status text NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'success', 'failed', 'cancelled')),
    backup_type text NOT NULL CHECK (backup_type IN ('database', 'storage', 'full')),
    object_key text,
    byte_size bigint CHECK (byte_size IS NULL OR byte_size >= 0),
    checksum_sha256 text,
    started_at timestamptz NOT NULL DEFAULT now(),
    finished_at timestamptz,
    error text
);

CREATE INDEX backup_runs_job_started_idx ON backup_runs (backup_job_id, started_at DESC);

CREATE TABLE system_settings (
    key text PRIMARY KEY,
    value jsonb NOT NULL,
    description text,
    updated_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER trg_system_settings_updated_at BEFORE UPDATE ON system_settings FOR EACH ROW EXECUTE FUNCTION set_updated_at();
