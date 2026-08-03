DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM notification_jobs WHERE status = 'skipped') THEN
        RAISE EXCEPTION
            'cannot roll back skipped push semantics while skipped jobs exist; restore a pre-migration snapshot or resolve those rows explicitly';
    END IF;
END
$$;

ALTER TABLE notification_jobs
    DROP CONSTRAINT IF EXISTS notification_jobs_status_check;

ALTER TABLE notification_jobs
    ADD CONSTRAINT notification_jobs_status_check
    CHECK (status IN ('pending', 'processing', 'sent', 'failed', 'dead'));

COMMENT ON COLUMN notification_jobs.status IS NULL;
