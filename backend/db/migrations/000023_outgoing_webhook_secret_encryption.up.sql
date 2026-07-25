ALTER TABLE outgoing_webhooks
    ADD COLUMN signing_secret_encrypted text;

-- The previous implementation discarded the signing key after returning it, so
-- existing webhooks cannot produce signatures verifiable with the issued secret.
UPDATE outgoing_webhooks
SET signing_secret_encrypted = 'legacy:v0:rotation-required',
    status = 'disabled';

ALTER TABLE outgoing_webhooks
    ALTER COLUMN signing_secret_encrypted SET NOT NULL,
    DROP COLUMN secret_hash;

COMMENT ON COLUMN outgoing_webhooks.signing_secret_encrypted IS
    'Versioned AES-GCM envelope containing the customer webhook signing secret.';
