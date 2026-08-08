-- User-generated-content safety controls required by the mobile store release.

CREATE TABLE moderation_reports (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    reporter_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    target_type text NOT NULL CHECK (target_type IN ('message', 'user')),
    target_id uuid NOT NULL,
    target_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    target_snapshot jsonb NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(target_snapshot) = 'object'),
    reason text NOT NULL CHECK (reason IN (
        'spam',
        'harassment',
        'hate_speech',
        'sexual_content',
        'violence',
        'illegal_content',
        'privacy',
        'impersonation',
        'other'
    )),
    details text,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'reviewing', 'resolved', 'dismissed')),
    resolution_note text,
    resolved_by uuid REFERENCES users (id) ON DELETE SET NULL,
    resolved_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (reporter_user_id IS NULL OR reporter_user_id <> target_user_id),
    CHECK (
        (status IN ('pending', 'reviewing') AND resolved_at IS NULL) OR
        (status IN ('resolved', 'dismissed') AND resolved_at IS NOT NULL)
    )
);

CREATE UNIQUE INDEX moderation_reports_active_duplicate_uidx
    ON moderation_reports (workspace_id, reporter_user_id, target_type, target_id)
    WHERE reporter_user_id IS NOT NULL AND status IN ('pending', 'reviewing');
CREATE INDEX moderation_reports_workspace_queue_idx
    ON moderation_reports (workspace_id, status, created_at DESC, id DESC);
CREATE INDEX moderation_reports_target_idx
    ON moderation_reports (workspace_id, target_type, target_id, created_at DESC);
CREATE INDEX moderation_reports_reporter_rate_idx
    ON moderation_reports (reporter_user_id, created_at DESC)
    WHERE reporter_user_id IS NOT NULL;
CREATE TRIGGER trg_moderation_reports_updated_at
    BEFORE UPDATE ON moderation_reports
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE user_blocks (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    blocker_user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    blocked_user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    reason text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (workspace_id, blocker_user_id, blocked_user_id),
    CHECK (blocker_user_id <> blocked_user_id),
    FOREIGN KEY (workspace_id, blocker_user_id)
        REFERENCES workspace_members (workspace_id, user_id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, blocked_user_id)
        REFERENCES workspace_members (workspace_id, user_id) ON DELETE CASCADE
);

CREATE INDEX user_blocks_blocker_idx
    ON user_blocks (workspace_id, blocker_user_id, created_at DESC);
CREATE INDEX user_blocks_blocked_idx
    ON user_blocks (workspace_id, blocked_user_id);
CREATE TRIGGER trg_user_blocks_updated_at
    BEFORE UPDATE ON user_blocks
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE user_legal_acceptances (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    document_type text NOT NULL CHECK (document_type IN ('terms', 'privacy')),
    document_version text NOT NULL CHECK (length(trim(document_version)) BETWEEN 1 AND 64),
    accepted_at timestamptz NOT NULL DEFAULT now(),
    ip_address inet,
    user_agent text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    UNIQUE (user_id, workspace_id, document_type, document_version)
);

CREATE INDEX user_legal_acceptances_user_idx
    ON user_legal_acceptances (user_id, workspace_id, accepted_at DESC);

INSERT INTO permissions (code, module, action, name, description)
VALUES (
    'moderation.manage',
    'moderation',
    'manage',
    'Quản lý kiểm duyệt',
    'Xem và xử lý báo cáo nội dung/người dùng trong workspace'
)
ON CONFLICT (code) DO UPDATE SET
    module = EXCLUDED.module,
    action = EXCLUDED.action,
    name = EXCLUDED.name,
    description = EXCLUDED.description;

INSERT INTO role_permissions (role_id, permission_id)
SELECT role.id, permission.id
FROM roles role
JOIN permissions permission ON permission.code = 'moderation.manage'
WHERE role.code IN ('workspace_owner', 'workspace_admin')
  AND role.is_system = true
  AND role.deleted_at IS NULL
ON CONFLICT DO NOTHING;
