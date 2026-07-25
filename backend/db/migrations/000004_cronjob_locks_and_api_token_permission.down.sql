-- Hoàn tác quyền quản lý API token riêng và lock cronjob.

DROP INDEX IF EXISTS cron_jobs_lock_idx;

ALTER TABLE cron_jobs DROP COLUMN IF EXISTS locked_by;
ALTER TABLE cron_jobs DROP COLUMN IF EXISTS locked_at;

DELETE FROM role_permissions
WHERE permission_id IN (
    SELECT id FROM permissions WHERE code = 'api_token.manage'
);

DELETE FROM permissions WHERE code = 'api_token.manage';
