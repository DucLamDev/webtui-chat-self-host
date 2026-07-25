WITH seeded_roles (code) AS (
    VALUES
        ('workspace_owner'),
        ('workspace_admin'),
        ('workspace_member')
),
seeded_permissions (code) AS (
    VALUES
        ('workspace.manage'),
        ('workspace.invite_user'),
        ('workspace.view_members'),
        ('role.manage'),
        ('channel.create'),
        ('channel.manage'),
        ('channel.delete'),
        ('message.send'),
        ('message.manage'),
        ('file.upload'),
        ('bot.manage'),
        ('webhook.manage'),
        ('module.manage'),
        ('audit.view'),
        ('backup.manage'),
        ('admin.view'),
        ('notification.manage'),
        ('cronjob.manage')
)
DELETE FROM role_permissions rp
USING roles r, permissions p, seeded_roles sr, seeded_permissions sp
WHERE rp.role_id = r.id
  AND rp.permission_id = p.id
  AND r.workspace_id IS NULL
  AND r.code = sr.code
  AND p.code = sp.code;

DELETE FROM roles
WHERE workspace_id IS NULL
  AND code IN ('workspace_owner', 'workspace_admin', 'workspace_member');

DELETE FROM permissions
WHERE code IN (
    'workspace.manage',
    'workspace.invite_user',
    'workspace.view_members',
    'role.manage',
    'channel.create',
    'channel.manage',
    'channel.delete',
    'message.send',
    'message.manage',
    'file.upload',
    'bot.manage',
    'webhook.manage',
    'module.manage',
    'audit.view',
    'backup.manage',
    'admin.view',
    'notification.manage',
    'cronjob.manage'
);
