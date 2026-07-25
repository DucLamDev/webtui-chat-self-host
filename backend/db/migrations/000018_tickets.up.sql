CREATE TABLE IF NOT EXISTS tickets (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id uuid NOT NULL REFERENCES workspaces (id) ON DELETE CASCADE,
    channel_id uuid REFERENCES channels (id) ON DELETE SET NULL,
    title text NOT NULL,
    description text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'pending', 'resolved', 'closed')),
    priority text NOT NULL DEFAULT 'normal' CHECK (priority IN ('low', 'normal', 'high', 'urgent')),
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    assigned_to uuid REFERENCES users (id) ON DELETE SET NULL,
    metadata jsonb NOT NULL DEFAULT '{}',
    resolved_at timestamptz,
    closed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    deleted_at timestamptz
);

CREATE INDEX IF NOT EXISTS tickets_workspace_status_idx
ON tickets (workspace_id, status, updated_at DESC)
WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS tickets_assignee_idx
ON tickets (assigned_to, status)
WHERE deleted_at IS NULL AND assigned_to IS NOT NULL;

CREATE TRIGGER trg_tickets_updated_at
BEFORE UPDATE ON tickets
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

INSERT INTO permissions (code, module, action, name, description)
VALUES
    ('ticket.view', 'ticket', 'view', 'Xem ticket', 'Xem va tao ticket ho tro trong workspace'),
    ('ticket.manage', 'ticket', 'manage', 'Quan ly ticket', 'Cap nhat trang thai, do uu tien va nguoi phu trach ticket')
ON CONFLICT (code) DO UPDATE
SET module = EXCLUDED.module,
    action = EXCLUDED.action,
    name = EXCLUDED.name,
    description = EXCLUDED.description;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code IN ('ticket.view', 'ticket.manage')
WHERE r.workspace_id IS NULL
  AND r.code IN ('workspace_owner', 'workspace_admin')
  AND r.deleted_at IS NULL
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code = 'ticket.view'
WHERE r.workspace_id IS NULL
  AND r.code = 'workspace_member'
  AND r.deleted_at IS NULL
ON CONFLICT DO NOTHING;
