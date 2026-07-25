package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	authapp "github.com/duclamdev/application-chat/backend/internal/modules/auth/application"
	authdomain "github.com/duclamdev/application-chat/backend/internal/modules/auth/domain"
	tenancydomain "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) ListPublicOIDCProviders(
	ctx context.Context,
	domain string,
) (authapp.ZoneAccess, []tenancydomain.OIDCProvider, error) {
	target, err := r.ResolveZoneAccess(ctx, domain)
	if err != nil {
		return authapp.ZoneAccess{}, nil, err
	}
	if target.ZoneStatus != "active" {
		return target, []tenancydomain.OIDCProvider{}, nil
	}
	rows, err := r.pool.Query(ctx, authOIDCProviderSelect+`
WHERE provider.zone_id = $1::uuid
  AND provider.status = 'configured'
  AND provider.deleted_at IS NULL
ORDER BY provider.name, provider.created_at
`, target.ZoneID)
	if err != nil {
		return authapp.ZoneAccess{}, nil, err
	}
	defer rows.Close()
	providers := make([]tenancydomain.OIDCProvider, 0)
	for rows.Next() {
		provider, scanErr := scanAuthOIDCProvider(rows)
		if scanErr != nil {
			return authapp.ZoneAccess{}, nil, scanErr
		}
		providers = append(providers, provider)
	}
	return target, providers, rows.Err()
}

func (r *Repository) ResolveOIDCProvider(
	ctx context.Context,
	domain string,
	providerID string,
) (authapp.ZoneAccess, tenancydomain.OIDCProvider, error) {
	target, err := r.ResolveZoneAccess(ctx, domain)
	if err != nil {
		return authapp.ZoneAccess{}, tenancydomain.OIDCProvider{}, err
	}
	if target.ZoneStatus != "active" {
		return authapp.ZoneAccess{}, tenancydomain.OIDCProvider{}, tenancydomain.ErrOIDCProviderNotFound
	}
	provider, err := scanAuthOIDCProvider(r.pool.QueryRow(ctx, authOIDCProviderSelect+`
WHERE provider.id = $1::uuid
  AND provider.zone_id = $2::uuid
  AND provider.status = 'configured'
  AND provider.deleted_at IS NULL
`, strings.TrimSpace(providerID), target.ZoneID))
	if errors.Is(err, pgx.ErrNoRows) {
		return authapp.ZoneAccess{}, tenancydomain.OIDCProvider{}, tenancydomain.ErrOIDCProviderNotFound
	}
	return target, provider, err
}

func (r *Repository) CreateOIDCLoginState(
	ctx context.Context,
	params authapp.CreateOIDCLoginStateParams,
) error {
	_, _ = r.pool.Exec(ctx, `
DELETE FROM zone_oidc_login_states
WHERE expires_at < now() - interval '1 hour'
   OR consumed_at < now() - interval '1 hour'
`)
	_, err := r.pool.Exec(ctx, `
INSERT INTO zone_oidc_login_states (
    state_hash, provider_id, zone_id, domain, redirect_uri, return_to,
    code_verifier_encrypted, nonce, device_name, ip_address, user_agent, expires_at
)
VALUES (
    $1, $2::uuid, $3::uuid, $4, $5, $6, $7, $8,
    NULLIF($9, ''), NULLIF($10, '')::inet, NULLIF($11, ''), $12
)
`, params.StateHash, params.ProviderID, params.ZoneID, params.Domain, params.RedirectURI,
		params.ReturnTo, params.CodeVerifierEncrypted, params.Nonce, params.DeviceName,
		params.IPAddress, params.UserAgent, params.ExpiresAt)
	return err
}

func (r *Repository) ConsumeOIDCLoginState(
	ctx context.Context,
	stateHash []byte,
	now time.Time,
) (authapp.OIDCLoginState, error) {
	var state authapp.OIDCLoginState
	var providerID string
	var zoneID string
	var domain string
	var deviceName sql.NullString
	var ipAddress sql.NullString
	var userAgent sql.NullString
	err := r.pool.QueryRow(ctx, `
UPDATE zone_oidc_login_states
SET consumed_at = $2
WHERE state_hash = $1
  AND consumed_at IS NULL
  AND expires_at > $2
RETURNING provider_id::text, zone_id::text, domain, redirect_uri, return_to,
          code_verifier_encrypted, nonce, device_name, host(ip_address), user_agent
`, stateHash, now).Scan(
		&providerID,
		&zoneID,
		&domain,
		&state.RedirectURI,
		&state.ReturnTo,
		&state.CodeVerifierEncrypted,
		&state.Nonce,
		&deviceName,
		&ipAddress,
		&userAgent,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return authapp.OIDCLoginState{}, authapp.ErrOIDCStateNotFound
	}
	if err != nil {
		return authapp.OIDCLoginState{}, err
	}
	target, provider, err := r.ResolveOIDCProvider(ctx, domain, providerID)
	if err != nil || target.ZoneID != zoneID {
		if err == nil {
			err = tenancydomain.ErrOIDCProviderNotFound
		}
		return authapp.OIDCLoginState{}, err
	}
	state.Provider = provider
	state.Target = target
	state.DeviceName = deviceName.String
	state.IPAddress = ipAddress.String
	state.UserAgent = userAgent.String
	return state, nil
}

func (r *Repository) CreateOIDCLoginResult(
	ctx context.Context,
	params authapp.CreateOIDCLoginResultParams,
) error {
	claims, err := json.Marshal(params.Claims)
	if err != nil {
		return err
	}
	_, _ = r.pool.Exec(ctx, `
DELETE FROM zone_oidc_login_results
WHERE expires_at < now() - interval '1 hour'
   OR consumed_at < now() - interval '1 hour'
`)
	_, err = r.pool.Exec(ctx, `
INSERT INTO zone_oidc_login_results (
    code_hash, provider_id, zone_id, domain, claims,
    device_name, ip_address, user_agent, expires_at
)
VALUES (
    $1, $2::uuid, $3::uuid, $4, $5::jsonb,
    NULLIF($6, ''), NULLIF($7, '')::inet, NULLIF($8, ''), $9
)
`, params.CodeHash, params.ProviderID, params.ZoneID, params.Domain, string(claims),
		params.DeviceName, params.IPAddress, params.UserAgent, params.ExpiresAt)
	return err
}

func (r *Repository) ConsumeOIDCLoginResult(
	ctx context.Context,
	codeHash []byte,
	domain string,
	now time.Time,
) (authapp.OIDCLoginResult, error) {
	var result authapp.OIDCLoginResult
	var providerID string
	var zoneID string
	var storedDomain string
	var rawClaims []byte
	var deviceName sql.NullString
	var ipAddress sql.NullString
	var userAgent sql.NullString
	err := r.pool.QueryRow(ctx, `
UPDATE zone_oidc_login_results
SET consumed_at = $3
WHERE code_hash = $1
  AND lower(domain) = lower($2)
  AND consumed_at IS NULL
  AND expires_at > $3
RETURNING provider_id::text, zone_id::text, domain, claims,
          device_name, host(ip_address), user_agent
`, codeHash, domain, now).Scan(
		&providerID,
		&zoneID,
		&storedDomain,
		&rawClaims,
		&deviceName,
		&ipAddress,
		&userAgent,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return authapp.OIDCLoginResult{}, authapp.ErrOIDCResultNotFound
	}
	if err != nil {
		return authapp.OIDCLoginResult{}, err
	}
	target, provider, err := r.ResolveOIDCProvider(ctx, storedDomain, providerID)
	if err != nil || target.ZoneID != zoneID {
		if err == nil {
			err = tenancydomain.ErrOIDCProviderNotFound
		}
		return authapp.OIDCLoginResult{}, err
	}
	if err := json.Unmarshal(rawClaims, &result.Claims); err != nil {
		return authapp.OIDCLoginResult{}, err
	}
	result.Provider = provider
	result.Target = target
	result.DeviceName = deviceName.String
	result.IPAddress = ipAddress.String
	result.UserAgent = userAgent.String
	return result, nil
}

func (r *Repository) ResolveOIDCUser(
	ctx context.Context,
	params authapp.ResolveOIDCUserParams,
) (authdomain.User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return authdomain.User{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	if err := validateZoneTarget(ctx, tx, params.Target); err != nil {
		return authdomain.User{}, err
	}
	var jitProvisioning bool
	err = tx.QueryRow(ctx, `
SELECT jit_provisioning
FROM zone_oidc_providers
WHERE id = $1::uuid
  AND zone_id = $2::uuid
  AND status = 'configured'
  AND deleted_at IS NULL
FOR UPDATE
`, params.Provider.ID, params.Target.ZoneID).Scan(&jitProvisioning)
	if errors.Is(err, pgx.ErrNoRows) {
		return authdomain.User{}, tenancydomain.ErrOIDCProviderNotFound
	}
	if err != nil {
		return authdomain.User{}, err
	}
	user, err := findOIDCIdentityUser(ctx, tx, params.Provider.ID, params.Subject)
	if err == nil {
		if err := updateOIDCIdentity(ctx, tx, params, user.ID); err != nil {
			return authdomain.User{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return authdomain.User{}, err
		}
		return user, nil
	}
	if !errors.Is(err, authapp.ErrOIDCIdentityNotFound) {
		return authdomain.User{}, err
	}

	if params.EmailVerified {
		user, err = findUserByEmail(ctx, tx, params.Email)
	} else {
		err = authdomain.ErrUserNotFound
	}
	switch {
	case err == nil:
		if !jitProvisioning {
			if err := requireActiveWorkspaceMembership(ctx, tx, user.ID, params.Target.WorkspaceID); err != nil {
				return authdomain.User{}, authapp.ErrOIDCJITDisabled
			}
		}
	case errors.Is(err, authdomain.ErrUserNotFound):
		if !jitProvisioning {
			return authdomain.User{}, authapp.ErrOIDCJITDisabled
		}
		user, err = createOIDCUser(ctx, tx, params)
		if err != nil {
			return authdomain.User{}, err
		}
	default:
		return authdomain.User{}, err
	}
	if user.Status != "active" {
		return authdomain.User{}, authdomain.ErrZoneAccessDenied
	}
	if err := ensureOIDCWorkspaceMembership(
		ctx, tx, user.ID, params.Target.WorkspaceID, jitProvisioning,
	); err != nil {
		return authdomain.User{}, err
	}
	claims, err := json.Marshal(params.Claims)
	if err != nil {
		return authdomain.User{}, err
	}
	commandTag, err := tx.Exec(ctx, `
INSERT INTO zone_oidc_identities (
    zone_id, provider_id, user_id, subject, email_at_link, last_claims, last_login_at
)
VALUES ($1::uuid, $2::uuid, $3::uuid, $4, $5, $6::jsonb, $7)
ON CONFLICT (provider_id, subject) DO NOTHING
`, params.Target.ZoneID, params.Provider.ID, user.ID, params.Subject, params.Email,
		string(claims), params.SeenAt)
	if err != nil {
		if isUniqueViolation(err) {
			return authdomain.User{}, authdomain.ErrUserAlreadyExists
		}
		return authdomain.User{}, err
	}
	if commandTag.RowsAffected() != 1 {
		return authdomain.User{}, authdomain.ErrUserAlreadyExists
	}
	if err := tx.Commit(ctx); err != nil {
		return authdomain.User{}, err
	}
	return user, nil
}

func findOIDCIdentityUser(
	ctx context.Context,
	tx pgx.Tx,
	providerID string,
	subject string,
) (authdomain.User, error) {
	user, err := scanUser(tx.QueryRow(ctx, userSelect+`
JOIN zone_oidc_identities identity ON identity.user_id = users.id
WHERE identity.provider_id = $1::uuid
  AND identity.subject = $2
  AND users.deleted_at IS NULL
FOR UPDATE OF identity, users
`, providerID, subject))
	if errors.Is(err, pgx.ErrNoRows) {
		return authdomain.User{}, authapp.ErrOIDCIdentityNotFound
	}
	return user, err
}

const userSelect = `
SELECT users.id::text, users.email::text, users.username::text, users.display_name,
       users.password_hash, users.avatar_url, users.status, users.locale, users.timezone,
       users.email_verified_at, users.last_seen_at, host(users.registration_ip_address),
       users.registration_device_name, host(users.last_ip_address), users.device_name,
       users.created_at, users.updated_at
FROM users
`

func findUserByEmail(ctx context.Context, tx pgx.Tx, email string) (authdomain.User, error) {
	user, err := scanUser(tx.QueryRow(ctx, userSelect+`
WHERE lower(users.email::text) = lower($1)
  AND users.deleted_at IS NULL
FOR UPDATE
`, email))
	if errors.Is(err, pgx.ErrNoRows) {
		return authdomain.User{}, authdomain.ErrUserNotFound
	}
	return user, err
}

func createOIDCUser(
	ctx context.Context,
	tx pgx.Tx,
	params authapp.ResolveOIDCUserParams,
) (authdomain.User, error) {
	user, err := scanUser(tx.QueryRow(ctx, `
INSERT INTO users (
    email, username, display_name, password_hash, status,
    registration_ip_address, registration_device_name, registration_user_agent,
    last_ip_address, device_name, last_user_agent, last_seen_at, email_verified_at
)
VALUES (
    $1, $2, $3, $4, 'active',
    NULLIF($5, '')::inet, NULLIF($6, ''), NULLIF($7, ''),
    NULLIF($5, '')::inet, NULLIF($6, ''), NULLIF($7, ''), $8,
    CASE WHEN $9 THEN $8 ELSE NULL END
)
RETURNING id::text, email::text, username::text, display_name, password_hash, avatar_url, status,
          locale, timezone, email_verified_at, last_seen_at,
          host(registration_ip_address), registration_device_name,
          host(last_ip_address), device_name,
          created_at, updated_at
`, params.Email, params.Username, params.DisplayName, params.PasswordHash,
		params.IPAddress, params.DeviceName, params.UserAgent, params.SeenAt,
		params.EmailVerified))
	if isUniqueViolation(err) {
		return authdomain.User{}, authdomain.ErrUserAlreadyExists
	}
	return user, err
}

func ensureOIDCWorkspaceMembership(
	ctx context.Context,
	tx pgx.Tx,
	userID string,
	workspaceID string,
	jitProvisioning bool,
) error {
	err := requireActiveWorkspaceMembership(ctx, tx, userID, workspaceID)
	if err == nil {
		return repairWorkspaceAccess(ctx, tx, userID, workspaceID)
	}
	if !errors.Is(err, authdomain.ErrZoneAccessDenied) || !jitProvisioning {
		return err
	}
	return addWorkspaceMembership(ctx, tx, userID, workspaceID, "")
}

func requireActiveWorkspaceMembership(
	ctx context.Context,
	tx pgx.Tx,
	userID string,
	workspaceID string,
) error {
	var status string
	err := tx.QueryRow(ctx, `
SELECT status
FROM workspace_members
WHERE workspace_id = $1::uuid
  AND user_id = $2::uuid
FOR UPDATE
`, workspaceID, userID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && status != "active") {
		return authdomain.ErrZoneAccessDenied
	}
	return err
}

func updateOIDCIdentity(
	ctx context.Context,
	tx pgx.Tx,
	params authapp.ResolveOIDCUserParams,
	userID string,
) error {
	if err := requireActiveWorkspaceMembership(ctx, tx, userID, params.Target.WorkspaceID); err != nil {
		return err
	}
	claims, err := json.Marshal(params.Claims)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
UPDATE zone_oidc_identities
SET last_claims = $4::jsonb,
    last_login_at = $5
WHERE provider_id = $1::uuid
  AND zone_id = $2::uuid
  AND user_id = $3::uuid
`, params.Provider.ID, params.Target.ZoneID, userID, string(claims), params.SeenAt)
	return err
}

const authOIDCProviderSelect = `
SELECT provider.id::text, provider.zone_id::text, provider.name,
       provider.issuer_url, provider.client_id, provider.client_secret_ref,
       provider.scopes, provider.claim_mapping, provider.jit_provisioning,
       provider.require_verified_email, provider.status,
       provider.created_by::text, provider.created_at, provider.updated_at
FROM zone_oidc_providers provider
`

func scanAuthOIDCProvider(row rowScanner) (tenancydomain.OIDCProvider, error) {
	var provider tenancydomain.OIDCProvider
	var clientSecretRef sql.NullString
	var createdBy sql.NullString
	var claimMapping []byte
	err := row.Scan(
		&provider.ID,
		&provider.ZoneID,
		&provider.Name,
		&provider.IssuerURL,
		&provider.ClientID,
		&clientSecretRef,
		&provider.Scopes,
		&claimMapping,
		&provider.JITProvisioning,
		&provider.RequireVerifiedEmail,
		&provider.Status,
		&createdBy,
		&provider.CreatedAt,
		&provider.UpdatedAt,
	)
	if err != nil {
		return tenancydomain.OIDCProvider{}, err
	}
	if clientSecretRef.Valid {
		provider.ClientSecretRef = &clientSecretRef.String
	}
	if createdBy.Valid {
		provider.CreatedBy = &createdBy.String
	}
	if err := json.Unmarshal(claimMapping, &provider.ClaimMapping); err != nil {
		return tenancydomain.OIDCProvider{}, err
	}
	return provider, nil
}

var _ authapp.OIDCRepository = (*Repository)(nil)
