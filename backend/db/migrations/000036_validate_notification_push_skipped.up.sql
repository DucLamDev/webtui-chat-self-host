-- The constraint was installed NOT VALID in the preceding migration so its
-- ACCESS EXCLUSIVE metadata lock was committed before this online validation.
-- Existing rows were already protected by the stricter previous constraint.
ALTER TABLE notification_jobs
    VALIDATE CONSTRAINT notification_jobs_status_check;
