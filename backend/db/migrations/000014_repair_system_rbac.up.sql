-- Keep the built-in RBAC roles deterministic for existing databases.
-- Owners and workspace admins are privileged roles; members only receive the
-- chat and self-service permissions explicitly listed below.

INSERT INTO workspace_members (workspace_id, user_id, status, joined_at)
SELECT w.id, w.owner_id, 'active', now()
FROM workspaces w
WHERE w.owner_id IS NOT NULL
  AND w.deleted_at IS NULL
ON CONFLICT (workspace_id, user_id)
DO UPDATE SET
    status = 'active',
    joined_at = COALESCE(workspace_members.joined_at, EXCLUDED.joined_at);

INSERT INTO workspace_member_roles (workspace_id, user_id, role_id, assigned_by)
SELECT w.id, w.owner_id, r.id, w.owner_id
FROM workspaces w
JOIN roles r
  ON r.workspace_id IS NULL
 AND r.is_system = true
 AND r.code = 'workspace_owner'
 AND r.deleted_at IS NULL
WHERE w.owner_id IS NOT NULL
  AND w.deleted_at IS NULL
ON CONFLICT (workspace_id, user_id, role_id) DO NOTHING;

-- Bootstrap the explicitly approved VPSTTT production administrator. This is
-- deliberately limited to one existing account and one existing workspace;
-- it creates nothing when either side is absent.
INSERT INTO workspace_members (workspace_id, user_id, status, joined_at)
SELECT w.id, u.id, 'active', now()
FROM workspaces w
JOIN users u ON u.email = 'admin@vpsttt.com' AND u.deleted_at IS NULL
WHERE w.id = '3f1e32b9-0a2f-4ca1-b0dc-04221a551c1c'::uuid
  AND w.deleted_at IS NULL
ON CONFLICT (workspace_id, user_id)
DO UPDATE SET
    status = 'active',
    joined_at = COALESCE(workspace_members.joined_at, EXCLUDED.joined_at);

INSERT INTO workspace_member_roles (workspace_id, user_id, role_id, assigned_by)
SELECT w.id, u.id, r.id, COALESCE(w.owner_id, u.id)
FROM workspaces w
JOIN users u ON u.email = 'admin@vpsttt.com' AND u.deleted_at IS NULL
JOIN roles r
  ON r.workspace_id IS NULL
 AND r.is_system = true
 AND r.code = 'workspace_admin'
 AND r.deleted_at IS NULL
WHERE w.id = '3f1e32b9-0a2f-4ca1-b0dc-04221a551c1c'::uuid
  AND w.deleted_at IS NULL
ON CONFLICT (workspace_id, user_id, role_id) DO NOTHING;

DELETE FROM role_permissions rp
USING roles r, permissions p
WHERE rp.role_id = r.id
  AND rp.permission_id = p.id
  AND r.workspace_id IS NULL
  AND r.is_system = true
  AND r.code = 'workspace_member'
  AND r.deleted_at IS NULL
  AND p.code NOT IN (
      'workspace.view_members',
      'message.send',
      'file.upload',
      'order.view',
      'order.payment_request'
  );

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.workspace_id IS NULL
  AND r.is_system = true
  AND r.code IN ('workspace_owner', 'workspace_admin')
  AND r.deleted_at IS NULL
ON CONFLICT DO NOTHING;
