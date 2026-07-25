-- Bổ sung quyền quản lý API token riêng và lock an toàn cho cronjob nhiều worker.

INSERT INTO permissions (code, module, action, name, description)
VALUES ('api_token.manage', 'api_token', 'manage', 'Quản lý API token', 'Tạo, xem và thu hồi API token')
ON CONFLICT (code) DO UPDATE
SET module = EXCLUDED.module,
    action = EXCLUDED.action,
    name = EXCLUDED.name,
    description = EXCLUDED.description;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.code = 'api_token.manage'
WHERE r.code IN ('workspace_owner', 'workspace_admin')
ON CONFLICT DO NOTHING;

ALTER TABLE cron_jobs ADD COLUMN IF NOT EXISTS locked_at timestamptz;
ALTER TABLE cron_jobs ADD COLUMN IF NOT EXISTS locked_by text;

CREATE INDEX IF NOT EXISTS cron_jobs_lock_idx ON cron_jobs (locked_at) WHERE status = 'active';
