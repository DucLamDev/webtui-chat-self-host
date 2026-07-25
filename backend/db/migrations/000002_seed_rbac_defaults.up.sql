-- Seed quyền và role hệ thống cho phase auth/RBAC.

WITH seed_permissions (code, module, action, name, description) AS (
    VALUES
        ('workspace.manage', 'workspace', 'manage', 'Quản lý workspace', 'Cấu hình và cập nhật workspace'),
        ('workspace.invite_user', 'workspace', 'invite_user', 'Mời người dùng', 'Mời thành viên vào workspace'),
        ('workspace.view_members', 'workspace', 'view_members', 'Xem thành viên', 'Xem danh sách thành viên workspace'),
        ('role.manage', 'role', 'manage', 'Quản lý role', 'Tạo, sửa và gán role'),
        ('channel.create', 'channel', 'create', 'Tạo kênh', 'Tạo channel public/private/direct'),
        ('channel.manage', 'channel', 'manage', 'Quản lý kênh', 'Cập nhật cấu hình và thành viên channel'),
        ('channel.delete', 'channel', 'delete', 'Xóa kênh', 'Xóa hoặc archive channel'),
        ('message.send', 'message', 'send', 'Gửi tin nhắn', 'Gửi message vào channel'),
        ('message.manage', 'message', 'manage', 'Quản lý tin nhắn', 'Sửa, xóa hoặc kiểm duyệt message'),
        ('file.upload', 'file', 'upload', 'Tải file lên', 'Upload file và gắn vào message'),
        ('bot.manage', 'bot', 'manage', 'Quản lý bot', 'Tạo và cấu hình bot'),
        ('webhook.manage', 'webhook', 'manage', 'Quản lý webhook', 'Tạo incoming/outgoing webhook'),
        ('module.manage', 'module', 'manage', 'Quản lý module', 'Quản lý module runner và script'),
        ('audit.view', 'audit', 'view', 'Xem audit log', 'Xem lịch sử thao tác hệ thống'),
        ('backup.manage', 'backup', 'manage', 'Quản lý backup', 'Tạo và kiểm tra backup'),
        ('admin.view', 'admin', 'view', 'Xem admin', 'Truy cập màn hình quản trị'),
        ('notification.manage', 'notification', 'manage', 'Quản lý notification', 'Cấu hình thông báo'),
        ('cronjob.manage', 'cronjob', 'manage', 'Quản lý cronjob', 'Tạo và vận hành cronjob')
)
INSERT INTO permissions (code, module, action, name, description)
SELECT code, module, action, name, description
FROM seed_permissions
ON CONFLICT (code) DO UPDATE
SET module = EXCLUDED.module,
    action = EXCLUDED.action,
    name = EXCLUDED.name,
    description = EXCLUDED.description;

WITH seed_roles (code, name, description) AS (
    VALUES
        ('workspace_owner', 'Chủ sở hữu workspace', 'Có toàn quyền trong workspace'),
        ('workspace_admin', 'Quản trị workspace', 'Quản trị hầu hết tài nguyên trong workspace'),
        ('workspace_member', 'Thành viên workspace', 'Dùng các chức năng chat cơ bản')
)
INSERT INTO roles (workspace_id, code, name, description, is_system)
SELECT NULL, code, name, description, true
FROM seed_roles sr
WHERE NOT EXISTS (
    SELECT 1
    FROM roles r
    WHERE r.workspace_id IS NULL
      AND r.code = sr.code
      AND r.deleted_at IS NULL
);

WITH role_permission (role_code, permission_code) AS (
    VALUES
        ('workspace_owner', 'workspace.manage'),
        ('workspace_owner', 'workspace.invite_user'),
        ('workspace_owner', 'workspace.view_members'),
        ('workspace_owner', 'role.manage'),
        ('workspace_owner', 'channel.create'),
        ('workspace_owner', 'channel.manage'),
        ('workspace_owner', 'channel.delete'),
        ('workspace_owner', 'message.send'),
        ('workspace_owner', 'message.manage'),
        ('workspace_owner', 'file.upload'),
        ('workspace_owner', 'bot.manage'),
        ('workspace_owner', 'webhook.manage'),
        ('workspace_owner', 'module.manage'),
        ('workspace_owner', 'audit.view'),
        ('workspace_owner', 'backup.manage'),
        ('workspace_owner', 'admin.view'),
        ('workspace_owner', 'notification.manage'),
        ('workspace_owner', 'cronjob.manage'),
        ('workspace_admin', 'workspace.invite_user'),
        ('workspace_admin', 'workspace.view_members'),
        ('workspace_admin', 'channel.create'),
        ('workspace_admin', 'channel.manage'),
        ('workspace_admin', 'message.send'),
        ('workspace_admin', 'message.manage'),
        ('workspace_admin', 'file.upload'),
        ('workspace_admin', 'bot.manage'),
        ('workspace_admin', 'webhook.manage'),
        ('workspace_admin', 'admin.view'),
        ('workspace_member', 'workspace.view_members'),
        ('workspace_member', 'message.send'),
        ('workspace_member', 'file.upload')
)
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM role_permission rp
JOIN roles r
  ON r.workspace_id IS NULL
 AND r.code = rp.role_code
 AND r.deleted_at IS NULL
JOIN permissions p
  ON p.code = rp.permission_code
ON CONFLICT DO NOTHING;
