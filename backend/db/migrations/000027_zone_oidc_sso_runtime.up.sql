ALTER TABLE zone_oidc_providers
    ADD COLUMN jit_provisioning boolean NOT NULL DEFAULT false,
    ADD COLUMN require_verified_email boolean NOT NULL DEFAULT true;

ALTER TABLE zone_oidc_providers
    ADD CONSTRAINT zone_oidc_providers_id_zone_unique UNIQUE (id, zone_id);

CREATE TABLE zone_oidc_login_states (
    state_hash bytea PRIMARY KEY CHECK (octet_length(state_hash) = 32),
    provider_id uuid NOT NULL,
    zone_id uuid NOT NULL REFERENCES zones (id) ON DELETE CASCADE,
    domain text NOT NULL,
    redirect_uri text NOT NULL,
    return_to text NOT NULL DEFAULT '/',
    code_verifier_encrypted text NOT NULL,
    nonce text NOT NULL,
    device_name text,
    ip_address inet,
    user_agent text,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (provider_id, zone_id)
        REFERENCES zone_oidc_providers (id, zone_id) ON DELETE CASCADE
);

CREATE INDEX zone_oidc_login_states_expiry_idx
    ON zone_oidc_login_states (expires_at)
    WHERE consumed_at IS NULL;

CREATE TABLE zone_oidc_login_results (
    code_hash bytea PRIMARY KEY CHECK (octet_length(code_hash) = 32),
    provider_id uuid NOT NULL,
    zone_id uuid NOT NULL REFERENCES zones (id) ON DELETE CASCADE,
    domain text NOT NULL,
    claims jsonb NOT NULL CHECK (jsonb_typeof(claims) = 'object'),
    device_name text,
    ip_address inet,
    user_agent text,
    expires_at timestamptz NOT NULL,
    consumed_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (provider_id, zone_id)
        REFERENCES zone_oidc_providers (id, zone_id) ON DELETE CASCADE
);

CREATE INDEX zone_oidc_login_results_expiry_idx
    ON zone_oidc_login_results (expires_at)
    WHERE consumed_at IS NULL;

CREATE TABLE zone_oidc_identities (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    zone_id uuid NOT NULL REFERENCES zones (id) ON DELETE CASCADE,
    provider_id uuid NOT NULL,
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    subject text NOT NULL CHECK (length(subject) BETWEEN 1 AND 1000),
    email_at_link text NOT NULL,
    last_claims jsonb NOT NULL DEFAULT '{}'::jsonb,
    last_login_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider_id, subject),
    UNIQUE (provider_id, user_id),
    FOREIGN KEY (provider_id, zone_id)
        REFERENCES zone_oidc_providers (id, zone_id) ON DELETE CASCADE
);

CREATE INDEX zone_oidc_identities_zone_user_idx
    ON zone_oidc_identities (zone_id, user_id);

CREATE TRIGGER trg_zone_oidc_identities_updated_at
    BEFORE UPDATE ON zone_oidc_identities
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

COMMENT ON TABLE zone_oidc_login_states IS
    'Short-lived, single-use OIDC state, nonce, and encrypted PKCE verifier records.';
COMMENT ON TABLE zone_oidc_login_results IS
    'Short-lived, single-use verified OIDC claims exchanged for a VPSTTT session.';
COMMENT ON TABLE zone_oidc_identities IS
    'Stable per-zone mapping from an OIDC issuer subject to a VPSTTT user.';
COMMENT ON TABLE zone_oidc_providers IS
    'Per-zone OIDC providers. Runtime readiness also requires OIDC_STATE_SECRET and a resolvable client_secret_ref.';
