CREATE TABLE notification_job_replays (
    original_job_id uuid PRIMARY KEY REFERENCES notification_jobs (id) ON DELETE CASCADE,
    replay_job_id uuid NOT NULL UNIQUE REFERENCES notification_jobs (id) ON DELETE CASCADE,
    actor_user_id uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    CHECK (original_job_id <> replay_job_id)
);

COMMENT ON TABLE notification_job_replays IS
    'Idempotent audit link for manual notification dead-letter replays. A replay that fails can itself be replayed.';
