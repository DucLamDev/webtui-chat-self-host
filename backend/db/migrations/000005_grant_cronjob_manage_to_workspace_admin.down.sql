-- Thu hồi quyền quản lý cronjob khỏi workspace admin.

DELETE FROM role_permissions rp
USING roles r, permissions p
WHERE rp.role_id = r.id
  AND rp.permission_id = p.id
  AND r.workspace_id IS NULL
  AND r.code = 'workspace_admin'
  AND p.code = 'cronjob.manage';
