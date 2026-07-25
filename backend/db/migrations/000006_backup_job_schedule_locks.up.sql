-- Bổ sung loại backup, lịch chạy và lock an toàn cho backup job.

ALTER TABLE backup_jobs ADD COLUMN IF NOT EXISTS backup_type text NOT NULL DEFAULT 'database' CHECK (backup_type IN ('database', 'storage', 'full'));
ALTER TABLE backup_jobs ADD COLUMN IF NOT EXISTS last_run_at timestamptz;
ALTER TABLE backup_jobs ADD COLUMN IF NOT EXISTS next_run_at timestamptz;
ALTER TABLE backup_jobs ADD COLUMN IF NOT EXISTS locked_at timestamptz;
ALTER TABLE backup_jobs ADD COLUMN IF NOT EXISTS locked_by text;

CREATE INDEX IF NOT EXISTS backup_jobs_due_idx ON backup_jobs (next_run_at) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS backup_jobs_lock_idx ON backup_jobs (locked_at) WHERE status = 'active';
