ALTER TABLE channel_guest_requests
    DROP CONSTRAINT IF EXISTS channel_guest_requests_legal_evidence_check,
    DROP COLUMN IF EXISTS legal_user_agent,
    DROP COLUMN IF EXISTS legal_ip_address,
    DROP COLUMN IF EXISTS legal_accepted_at,
    DROP COLUMN IF EXISTS privacy_policy_version,
    DROP COLUMN IF EXISTS terms_version;

