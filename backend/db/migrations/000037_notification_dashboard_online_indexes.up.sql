-- migrate:no-transaction
-- Each statement is executed and committed separately by MigrationRunner.
-- DROP first makes a retry recover from an INVALID index left by an interrupted
-- CREATE INDEX CONCURRENTLY.
SET lock_timeout = '30s';
-- migrate:statement-breakpoint
SET statement_timeout = '30min';
-- migrate:statement-breakpoint
DROP INDEX CONCURRENTLY IF EXISTS notification_jobs_push_sent_at_idx;
-- migrate:statement-breakpoint
CREATE INDEX CONCURRENTLY notification_jobs_push_sent_at_idx
    ON notification_jobs (workspace_id, sent_at DESC)
    WHERE channel = 'push' AND status = 'sent';
-- migrate:statement-breakpoint
DROP INDEX CONCURRENTLY IF EXISTS notification_job_deliveries_recent_idx;
-- migrate:statement-breakpoint
CREATE INDEX CONCURRENTLY notification_job_deliveries_recent_idx
    ON notification_job_deliveries (delivered_at DESC, job_id);
-- migrate:statement-breakpoint
DROP INDEX CONCURRENTLY IF EXISTS notification_jobs_push_dashboard_idx;
-- migrate:statement-breakpoint
CREATE INDEX CONCURRENTLY notification_jobs_push_dashboard_idx
    ON notification_jobs (workspace_id, status, updated_at DESC)
    WHERE channel = 'push';
