CREATE TABLE notification_job_deliveries (
    job_id uuid NOT NULL REFERENCES notification_jobs (id) ON DELETE CASCADE,
    device_id uuid NOT NULL REFERENCES push_devices (id) ON DELETE CASCADE,
    provider text NOT NULL CHECK (provider IN ('fcm', 'apns')),
    delivered_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (job_id, device_id)
);

CREATE INDEX notification_jobs_processing_lease_idx
    ON notification_jobs (updated_at, created_at)
    WHERE status = 'processing';

COMMENT ON TABLE notification_job_deliveries IS
    'Per-device push completion ledger used to avoid resending successful destinations when a multi-device job retries.';
