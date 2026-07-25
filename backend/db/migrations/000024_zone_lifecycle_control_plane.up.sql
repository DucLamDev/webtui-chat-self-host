ALTER TABLE zones
    ADD COLUMN suspended_at timestamptz,
    ADD COLUMN archived_at timestamptz,
    ADD COLUMN lifecycle_reason text;

CREATE UNIQUE INDEX zone_domains_one_primary_uidx
    ON zone_domains (zone_id)
    WHERE kind = 'primary'
      AND deleted_at IS NULL
      AND status <> 'suspended';

CREATE TABLE zone_deployment_requests (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    zone_id uuid NOT NULL REFERENCES zones (id) ON DELETE CASCADE,
    requested_mode text NOT NULL
        CHECK (requested_mode IN ('shared', 'dedicated_compose', 'dedicated_k8s')),
    requested_database_mode text NOT NULL
        CHECK (requested_database_mode IN ('shared_schema', 'dedicated_schema', 'dedicated_database')),
    status text NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'provisioning', 'ready', 'failed', 'cancelled')),
    idempotency_key text NOT NULL,
    requested_by uuid REFERENCES users (id) ON DELETE SET NULL,
    reviewed_by uuid REFERENCES users (id) ON DELETE SET NULL,
    failure_reason text,
    metadata jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    completed_at timestamptz
);

CREATE UNIQUE INDEX zone_deployment_requests_idempotency_uidx
    ON zone_deployment_requests (zone_id, idempotency_key);
CREATE INDEX zone_deployment_requests_pending_idx
    ON zone_deployment_requests (status, created_at)
    WHERE status IN ('pending', 'approved', 'provisioning');
CREATE TRIGGER trg_zone_deployment_requests_updated_at
    BEFORE UPDATE ON zone_deployment_requests
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
