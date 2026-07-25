DROP TRIGGER IF EXISTS trg_zone_oidc_identities_updated_at ON zone_oidc_identities;
DROP TABLE IF EXISTS zone_oidc_identities;
DROP TABLE IF EXISTS zone_oidc_login_results;
DROP TABLE IF EXISTS zone_oidc_login_states;

ALTER TABLE zone_oidc_providers
    DROP CONSTRAINT IF EXISTS zone_oidc_providers_id_zone_unique,
    DROP COLUMN IF EXISTS require_verified_email,
    DROP COLUMN IF EXISTS jit_provisioning;

COMMENT ON TABLE zone_oidc_providers IS
    'Per-zone OIDC configuration contract. Login remains unavailable until a provider verifier is deployed.';
