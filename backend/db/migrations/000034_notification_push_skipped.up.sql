ALTER TABLE notification_jobs
    DROP CONSTRAINT IF EXISTS notification_jobs_status_check;

ALTER TABLE notification_jobs
    ADD CONSTRAINT notification_jobs_status_check
    CHECK (status IN ('pending', 'processing', 'sent', 'skipped', 'failed', 'dead'))
    NOT VALID;

COMMENT ON COLUMN notification_jobs.status IS
    'skipped means the durable in-app notification was retained but no eligible push destination existed.';
