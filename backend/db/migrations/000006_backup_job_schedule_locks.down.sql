-- Hoàn tác lịch chạy và lock backup job.

DROP INDEX IF EXISTS backup_jobs_lock_idx;
DROP INDEX IF EXISTS backup_jobs_due_idx;

ALTER TABLE backup_jobs DROP COLUMN IF EXISTS locked_by;
ALTER TABLE backup_jobs DROP COLUMN IF EXISTS locked_at;
ALTER TABLE backup_jobs DROP COLUMN IF EXISTS next_run_at;
ALTER TABLE backup_jobs DROP COLUMN IF EXISTS last_run_at;
ALTER TABLE backup_jobs DROP COLUMN IF EXISTS backup_type;
