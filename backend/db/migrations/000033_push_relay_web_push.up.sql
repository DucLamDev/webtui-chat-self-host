-- Browser subscriptions are intentionally separate from native push_devices:
-- a Web Push subscription has three credential fields and a different revoke
-- lifecycle from FCM/APNs device tokens.
CREATE TABLE web_push_subscriptions (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    zone_id uuid NOT NULL REFERENCES zones (id) ON DELETE CASCADE,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    endpoint text NOT NULL CHECK (char_length(endpoint) BETWEEN 12 AND 4096),
    endpoint_hash bytea GENERATED ALWAYS AS (digest(endpoint, 'sha256')) STORED,
    p256dh text NOT NULL CHECK (char_length(p256dh) BETWEEN 16 AND 256),
    auth_secret text NOT NULL CHECK (char_length(auth_secret) BETWEEN 8 AND 256),
    expiration_time timestamptz,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'revoked')),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    revoked_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT web_push_subscriptions_endpoint_unique UNIQUE (endpoint_hash)
);

CREATE INDEX web_push_subscriptions_account_active_idx
    ON web_push_subscriptions (zone_id, user_id, updated_at DESC)
    WHERE status = 'active';
CREATE TRIGGER trg_web_push_subscriptions_updated_at
    BEFORE UPDATE ON web_push_subscriptions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE notification_web_push_deliveries (
    job_id uuid NOT NULL REFERENCES notification_jobs (id) ON DELETE CASCADE,
    subscription_id uuid NOT NULL REFERENCES web_push_subscriptions (id) ON DELETE CASCADE,
    delivered_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (job_id, subscription_id)
);

CREATE INDEX notification_web_push_deliveries_recent_idx
    ON notification_web_push_deliveries (delivered_at DESC, job_id);

-- Durable queue for the standalone publisher relay. Device destinations are
-- never written to application logs; the database is the only retry source.
CREATE TABLE push_relay_jobs (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    publisher_id text NOT NULL CHECK (char_length(publisher_id) BETWEEN 1 AND 128),
    idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 8 AND 200),
    request_hash bytea NOT NULL CHECK (octet_length(request_hash) = 32),
    provider text NOT NULL CHECK (provider IN ('fcm', 'apns')),
    device_token text NOT NULL CHECK (char_length(device_token) BETWEEN 8 AND 4096),
    payload jsonb NOT NULL CHECK (jsonb_typeof(payload) = 'object'),
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'retry', 'sent', 'dead')),
    attempt_count integer NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    max_attempts integer NOT NULL DEFAULT 8 CHECK (max_attempts BETWEEN 1 AND 32),
    next_attempt_at timestamptz NOT NULL DEFAULT now(),
    leased_at timestamptz,
    delivered_at timestamptz,
    last_error text,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT push_relay_jobs_publisher_idempotency_unique
        UNIQUE (publisher_id, idempotency_key)
);

CREATE INDEX push_relay_jobs_due_idx
    ON push_relay_jobs (next_attempt_at, created_at)
    WHERE status IN ('pending', 'retry');
CREATE INDEX push_relay_jobs_lease_idx
    ON push_relay_jobs (leased_at, created_at)
    WHERE status = 'processing';
CREATE INDEX push_relay_jobs_publisher_created_idx
    ON push_relay_jobs (publisher_id, created_at DESC);
CREATE INDEX push_relay_jobs_sent_metrics_idx
    ON push_relay_jobs (delivered_at DESC)
    WHERE status = 'sent';
CREATE INDEX push_relay_jobs_dead_metrics_idx
    ON push_relay_jobs (updated_at DESC)
    WHERE status = 'dead';
CREATE TRIGGER trg_push_relay_jobs_updated_at
    BEFORE UPDATE ON push_relay_jobs
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
