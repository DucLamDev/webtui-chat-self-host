-- Public-room guests can create chat/audio/video UGC without an account. Keep
-- their explicit policy acceptance with the guest grant and revoke every
-- pre-migration grant instead of fabricating consent for it.

ALTER TABLE channel_guest_requests
    ADD COLUMN terms_version text,
    ADD COLUMN privacy_policy_version text,
    ADD COLUMN legal_accepted_at timestamptz,
    ADD COLUMN legal_ip_address inet,
    ADD COLUMN legal_user_agent text,
    ADD CONSTRAINT channel_guest_requests_legal_evidence_check CHECK (
        (
            terms_version IS NULL
            AND privacy_policy_version IS NULL
            AND legal_accepted_at IS NULL
            AND legal_ip_address IS NULL
            AND legal_user_agent IS NULL
        )
        OR
        (
            terms_version IS NOT NULL
            AND privacy_policy_version IS NOT NULL
            AND length(trim(terms_version)) BETWEEN 1 AND 64
            AND length(trim(privacy_policy_version)) BETWEEN 1 AND 64
            AND legal_accepted_at IS NOT NULL
        )
    );

UPDATE channel_guest_requests
SET status = 'expired'
WHERE status IN ('waiting', 'approved')
  AND legal_accepted_at IS NULL;
