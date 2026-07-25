ALTER TABLE outgoing_webhooks
    ADD COLUMN secret_hash text;

UPDATE outgoing_webhooks
SET secret_hash = encode(digest(signing_secret_encrypted, 'sha256'), 'hex');

ALTER TABLE outgoing_webhooks
    ALTER COLUMN secret_hash SET NOT NULL,
    DROP COLUMN signing_secret_encrypted;
