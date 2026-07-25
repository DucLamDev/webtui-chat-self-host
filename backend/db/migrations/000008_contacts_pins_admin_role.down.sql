INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code = 'admin.view'
WHERE r.workspace_id IS NULL
  AND r.code = 'workspace_owner'
  AND r.deleted_at IS NULL
ON CONFLICT DO NOTHING;

DELETE FROM role_permissions rp
USING roles r, permissions p
WHERE rp.role_id = r.id
  AND rp.permission_id = p.id
  AND p.code = 'user.manage';

DELETE FROM permissions
WHERE code = 'user.manage';

DROP TABLE IF EXISTS message_pins;
DROP TABLE IF EXISTS contact_requests;
