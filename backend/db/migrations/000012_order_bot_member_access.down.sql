-- Keep owner/admin permissions from Phase 1 and only revoke the member access
-- introduced by this migration.

DELETE FROM role_permissions rp
USING roles r, permissions p
WHERE rp.role_id = r.id
  AND rp.permission_id = p.id
  AND r.code = 'workspace_member'
  AND p.code = 'order.view';
