DELETE FROM role_permissions
USING permissions
WHERE role_permissions.permission_id = permissions.id
  AND permissions.code = 'moderation.manage';

DELETE FROM permissions WHERE code = 'moderation.manage';

DROP TABLE IF EXISTS user_legal_acceptances;
DROP TABLE IF EXISTS user_blocks;
DROP TABLE IF EXISTS moderation_reports;
