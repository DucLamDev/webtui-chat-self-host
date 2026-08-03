CREATE TABLE zone_storage_configs (
    zone_id uuid PRIMARY KEY REFERENCES zones (id) ON DELETE CASCADE,
    provider text NOT NULL DEFAULT 'local'
        CHECK (provider IN ('local', 'minio', 's3')),
    endpoint text,
    region text,
    bucket text,
    access_key_id text,
    secret_access_key_encrypted text,
    created_by uuid REFERENCES users (id) ON DELETE SET NULL,
    updated_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (
        provider = 'local'
        OR (
            endpoint IS NOT NULL AND btrim(endpoint) <> ''
            AND bucket IS NOT NULL AND btrim(bucket) <> ''
            AND access_key_id IS NOT NULL AND btrim(access_key_id) <> ''
            AND secret_access_key_encrypted IS NOT NULL AND btrim(secret_access_key_encrypted) <> ''
        )
    )
);

COMMENT ON TABLE zone_storage_configs IS
    'One isolated object-storage configuration per customer zone/host. Secrets are AES-GCM envelopes.';
COMMENT ON COLUMN zone_storage_configs.secret_access_key_encrypted IS
    'Versioned AES-GCM envelope; plaintext S3 credentials must never be stored or returned by the API.';

CREATE TRIGGER trg_zone_storage_configs_updated_at
BEFORE UPDATE ON zone_storage_configs
FOR EACH ROW EXECUTE FUNCTION set_updated_at();

