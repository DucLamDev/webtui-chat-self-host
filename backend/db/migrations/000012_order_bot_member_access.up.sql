-- Allow normal workspace members to use read-only order lookup bots.
-- Billing actions remain restricted to workspace owners and admins.

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p
  ON p.code = 'order.view'
WHERE r.code IN ('workspace_owner', 'workspace_admin', 'workspace_member')
  AND r.deleted_at IS NULL
ON CONFLICT DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p
  ON p.code = 'order.billing'
WHERE r.code IN ('workspace_owner', 'workspace_admin')
  AND r.deleted_at IS NULL
ON CONFLICT DO NOTHING;
