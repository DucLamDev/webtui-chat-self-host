-- migrate:no-transaction
SET lock_timeout = '30s';
-- migrate:statement-breakpoint
SET statement_timeout = '30min';
-- migrate:statement-breakpoint
DROP INDEX CONCURRENTLY IF EXISTS notification_jobs_push_dashboard_idx;
-- migrate:statement-breakpoint
DROP INDEX CONCURRENTLY IF EXISTS notification_job_deliveries_recent_idx;
-- migrate:statement-breakpoint
DROP INDEX CONCURRENTLY IF EXISTS notification_jobs_push_sent_at_idx;
