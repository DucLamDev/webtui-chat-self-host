-- Cho phép workspace admin quản lý cronjob vận hành.

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code = 'cronjob.manage'
WHERE r.workspace_id IS NULL
  AND r.code = 'workspace_admin'
  AND r.deleted_at IS NULL
ON CONFLICT DO NOTHING;
