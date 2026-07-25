CREATE TABLE contact_requests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    receiver_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    status text NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected', 'cancelled')),
    requested_at timestamptz NOT NULL DEFAULT now(),
    responded_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz,
    CHECK (requester_id <> receiver_id)
);

CREATE UNIQUE INDEX contact_requests_pair_uidx
ON contact_requests (LEAST(requester_id, receiver_id), GREATEST(requester_id, receiver_id))
WHERE deleted_at IS NULL AND status IN ('pending', 'accepted');

CREATE INDEX contact_requests_requester_idx ON contact_requests (requester_id, status, updated_at DESC);
CREATE INDEX contact_requests_receiver_idx ON contact_requests (receiver_id, status, updated_at DESC);
CREATE TRIGGER trg_contact_requests_updated_at BEFORE UPDATE ON contact_requests FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE message_pins (
    workspace_id uuid NOT NULL,
    channel_id uuid NOT NULL REFERENCES channels (id) ON DELETE CASCADE,
    message_id uuid NOT NULL,
    pinned_by uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (workspace_id, channel_id, message_id),
    FOREIGN KEY (workspace_id, message_id) REFERENCES messages (workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX message_pins_channel_created_idx ON message_pins (workspace_id, channel_id, created_at DESC);
CREATE INDEX message_pins_pinned_by_idx ON message_pins (pinned_by, created_at DESC);

INSERT INTO permissions (code, module, action, name, description)
VALUES ('user.manage', 'user', 'manage', 'Quản lý người dùng', 'Cập nhật trạng thái và hồ sơ người dùng trong admin')
ON CONFLICT (code) DO UPDATE
SET module = EXCLUDED.module,
    action = EXCLUDED.action,
    name = EXCLUDED.name,
    description = EXCLUDED.description;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code = 'user.manage'
WHERE r.workspace_id IS NULL
  AND r.code = 'workspace_admin'
  AND r.deleted_at IS NULL
ON CONFLICT DO NOTHING;

DELETE FROM role_permissions rp
USING roles r, permissions p
WHERE rp.role_id = r.id
  AND rp.permission_id = p.id
  AND r.workspace_id IS NULL
  AND r.code = 'workspace_owner'
  AND p.code = 'admin.view';
