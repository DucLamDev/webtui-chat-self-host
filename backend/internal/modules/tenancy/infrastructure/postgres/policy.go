package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	tenancyapp "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/application"
	tenancydomain "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) GetZoneQuota(
	ctx context.Context,
	zoneID string,
) (tenancydomain.ZoneQuota, tenancydomain.ZoneUsage, error) {
	quota, err := getZoneQuota(ctx, r.pool, zoneID)
	if err != nil {
		return tenancydomain.ZoneQuota{}, tenancydomain.ZoneUsage{}, err
	}
	usage, err := getZoneUsage(ctx, r.pool, zoneID)
	return quota, usage, err
}

func (r *Repository) UpdateZoneQuota(
	ctx context.Context,
	params tenancyapp.UpdateZoneQuotaParams,
) (tenancydomain.ZoneQuota, tenancydomain.ZoneUsage, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return tenancydomain.ZoneQuota{}, tenancydomain.ZoneUsage{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	usage, err := getZoneUsage(ctx, tx, params.ZoneID)
	if err != nil {
		return tenancydomain.ZoneQuota{}, tenancydomain.ZoneUsage{}, err
	}
	if params.EnforcementMode == "hard" &&
		(usage.Workspaces > params.MaxWorkspaces ||
			usage.Members > params.MaxMembers ||
			usage.StorageBytes > params.MaxStorageBytes ||
			usage.AutomationInstallations > params.MaxAutomationInstallations ||
			usage.Webhooks > params.MaxWebhooks) {
		return tenancydomain.ZoneQuota{}, tenancydomain.ZoneUsage{}, tenancydomain.ErrZoneQuotaExceeded
	}

	row := tx.QueryRow(ctx, `
UPDATE zone_quotas quota
SET max_workspaces = $2,
    max_members = $3,
    max_storage_bytes = $4,
    max_automation_installations = $5,
    max_webhooks = $6,
    enforcement_mode = $7
FROM zones zone
WHERE quota.zone_id = $1::uuid
  AND zone.id = quota.zone_id
  AND zone.deleted_at IS NULL
RETURNING quota.zone_id::text, quota.max_workspaces, quota.max_members,
          quota.max_storage_bytes, quota.max_automation_installations,
          quota.max_webhooks, quota.enforcement_mode,
          quota.created_at, quota.updated_at
`, params.ZoneID, params.MaxWorkspaces, params.MaxMembers, params.MaxStorageBytes,
		params.MaxAutomationInstallations, params.MaxWebhooks, params.EnforcementMode)
	quota, err := scanZoneQuota(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return tenancydomain.ZoneQuota{}, tenancydomain.ZoneUsage{}, tenancydomain.ErrZoneNotFound
	}
	if err != nil {
		return tenancydomain.ZoneQuota{}, tenancydomain.ZoneUsage{}, err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (zone_id, actor_user_id, action, entity_type, entity_id, metadata)
VALUES (
    $1::uuid, $2::uuid, 'zone.quota_updated', 'zone_quota', $1::uuid,
    jsonb_build_object(
        'max_workspaces', $3,
        'max_members', $4,
        'max_storage_bytes', $5,
        'max_automation_installations', $6,
        'max_webhooks', $7,
        'enforcement_mode', $8
    )
)
`, params.ZoneID, params.ActorUserID, params.MaxWorkspaces, params.MaxMembers,
		params.MaxStorageBytes, params.MaxAutomationInstallations, params.MaxWebhooks,
		params.EnforcementMode); err != nil {
		return tenancydomain.ZoneQuota{}, tenancydomain.ZoneUsage{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return tenancydomain.ZoneQuota{}, tenancydomain.ZoneUsage{}, err
	}
	return quota, usage, nil
}

func (r *Repository) ListOIDCProviders(
	ctx context.Context,
	zoneID string,
) ([]tenancydomain.OIDCProvider, error) {
	rows, err := r.pool.Query(ctx, oidcProviderSelect+`
WHERE provider.zone_id = $1::uuid
  AND provider.deleted_at IS NULL
ORDER BY provider.name, provider.created_at
`, zoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	providers := make([]tenancydomain.OIDCProvider, 0)
	for rows.Next() {
		provider, scanErr := scanOIDCProvider(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		providers = append(providers, provider)
	}
	return providers, rows.Err()
}

func (r *Repository) CreateOIDCProvider(
	ctx context.Context,
	params tenancyapp.CreateOIDCProviderParams,
) (tenancydomain.OIDCProvider, error) {
	claimMapping, err := json.Marshal(params.ClaimMapping)
	if err != nil {
		return tenancydomain.OIDCProvider{}, err
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return tenancydomain.OIDCProvider{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	row := tx.QueryRow(ctx, `
INSERT INTO zone_oidc_providers (
    zone_id, name, issuer_url, client_id, client_secret_ref,
    scopes, claim_mapping, jit_provisioning, require_verified_email,
    status, created_by
)
SELECT
    zone.id, $2, $3, $4, NULLIF($5, ''), $6::text[], $7::jsonb,
    $8, $9, $10, $11::uuid
FROM zones zone
WHERE zone.id = $1::uuid
  AND zone.status = 'active'
  AND zone.deleted_at IS NULL
RETURNING id::text, zone_id::text, name, issuer_url, client_id,
          client_secret_ref, scopes, claim_mapping, jit_provisioning,
          require_verified_email, status, created_by::text,
          created_at, updated_at
`, params.ZoneID, params.Name, params.IssuerURL, params.ClientID,
		params.ClientSecretRef, params.Scopes, string(claimMapping),
		params.JITProvisioning, params.RequireVerifiedEmail, params.Status,
		params.ActorUserID)
	provider, err := scanOIDCProvider(row)
	if err != nil {
		if isUniqueViolation(err) {
			return tenancydomain.OIDCProvider{}, tenancydomain.ErrOIDCProviderConflict
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return tenancydomain.OIDCProvider{}, tenancydomain.ErrZoneNotFound
		}
		return tenancydomain.OIDCProvider{}, err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (zone_id, actor_user_id, action, entity_type, entity_id, metadata)
VALUES (
    $1::uuid, $2::uuid, 'zone.oidc_provider_created',
    'zone_oidc_provider', $3::uuid,
    jsonb_build_object('name', $4, 'issuer_url', $5, 'status', $6)
)
`, params.ZoneID, params.ActorUserID, provider.ID, provider.Name,
		provider.IssuerURL, provider.Status); err != nil {
		return tenancydomain.OIDCProvider{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return tenancydomain.OIDCProvider{}, err
	}
	return provider, nil
}

func (r *Repository) UpdateOIDCProvider(
	ctx context.Context,
	params tenancyapp.UpdateOIDCProviderParams,
) (tenancydomain.OIDCProvider, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return tenancydomain.OIDCProvider{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	scopes := []string{}
	if params.Scopes != nil {
		scopes = *params.Scopes
	}
	claimMapping := []byte(`{}`)
	if params.ClaimMapping != nil {
		var err error
		claimMapping, err = json.Marshal(*params.ClaimMapping)
		if err != nil {
			return tenancydomain.OIDCProvider{}, err
		}
	}
	row := tx.QueryRow(ctx, `
UPDATE zone_oidc_providers
SET name = CASE WHEN $3::boolean THEN $4 ELSE name END,
    issuer_url = CASE WHEN $5::boolean THEN $6 ELSE issuer_url END,
    client_id = CASE WHEN $7::boolean THEN $8 ELSE client_id END,
    client_secret_ref = CASE WHEN $9::boolean THEN NULLIF($10, '') ELSE client_secret_ref END,
    scopes = CASE WHEN $11::boolean THEN $12::text[] ELSE scopes END,
    claim_mapping = CASE WHEN $13::boolean THEN $14::jsonb ELSE claim_mapping END,
    jit_provisioning = CASE WHEN $15::boolean THEN $16 ELSE jit_provisioning END,
    require_verified_email = CASE WHEN $17::boolean THEN $18 ELSE require_verified_email END,
    status = CASE WHEN $19::boolean THEN $20 ELSE status END
WHERE id = $1::uuid
  AND zone_id = $2::uuid
  AND deleted_at IS NULL
RETURNING id::text, zone_id::text, name, issuer_url, client_id,
          client_secret_ref, scopes, claim_mapping, jit_provisioning,
          require_verified_email, status, created_by::text,
          created_at, updated_at
`, params.ProviderID, params.ZoneID,
		params.Name != nil, stringValue(params.Name),
		params.IssuerURL != nil, stringValue(params.IssuerURL),
		params.ClientID != nil, stringValue(params.ClientID),
		params.ClientSecretRef != nil, stringValue(params.ClientSecretRef),
		params.Scopes != nil, scopes,
		params.ClaimMapping != nil, string(claimMapping),
		params.JITProvisioning != nil, boolValue(params.JITProvisioning),
		params.RequireVerifiedEmail != nil, boolValue(params.RequireVerifiedEmail),
		params.Status != nil, stringValue(params.Status))
	provider, err := scanOIDCProvider(row)
	if err != nil {
		if isUniqueViolation(err) {
			return tenancydomain.OIDCProvider{}, tenancydomain.ErrOIDCProviderConflict
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return tenancydomain.OIDCProvider{}, tenancydomain.ErrOIDCProviderNotFound
		}
		return tenancydomain.OIDCProvider{}, err
	}
	_, err = tx.Exec(ctx, `
INSERT INTO audit_logs (zone_id, actor_user_id, action, entity_type, entity_id, metadata)
VALUES (
    $1::uuid, $2::uuid, 'zone.oidc_provider_updated',
    'zone_oidc_provider', $3::uuid,
    jsonb_build_object('status', $4)
)
`, params.ZoneID, params.ActorUserID, provider.ID, provider.Status)
	if err != nil {
		return tenancydomain.OIDCProvider{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return tenancydomain.OIDCProvider{}, err
	}
	return provider, nil
}

func (r *Repository) DeleteOIDCProvider(
	ctx context.Context,
	zoneID string,
	providerID string,
	actorUserID string,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	var name string
	if err := tx.QueryRow(ctx, `
UPDATE zone_oidc_providers
SET status = 'disabled',
    deleted_at = now()
WHERE id = $1::uuid
  AND zone_id = $2::uuid
  AND deleted_at IS NULL
RETURNING name
`, providerID, zoneID).Scan(&name); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tenancydomain.ErrOIDCProviderNotFound
		}
		return err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (zone_id, actor_user_id, action, entity_type, entity_id, metadata)
VALUES (
    $1::uuid, $2::uuid, 'zone.oidc_provider_deleted',
    'zone_oidc_provider', $3::uuid,
    jsonb_build_object('name', $4)
)
`, zoneID, actorUserID, providerID, name); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

const oidcProviderSelect = `
SELECT provider.id::text, provider.zone_id::text, provider.name,
       provider.issuer_url, provider.client_id, provider.client_secret_ref,
       provider.scopes, provider.claim_mapping, provider.jit_provisioning,
       provider.require_verified_email, provider.status,
       provider.created_by::text, provider.created_at, provider.updated_at
FROM zone_oidc_providers provider
`

func getZoneQuota(
	ctx context.Context,
	query interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	},
	zoneID string,
) (tenancydomain.ZoneQuota, error) {
	row := query.QueryRow(ctx, `
SELECT quota.zone_id::text, quota.max_workspaces, quota.max_members,
       quota.max_storage_bytes, quota.max_automation_installations,
       quota.max_webhooks, quota.enforcement_mode,
       quota.created_at, quota.updated_at
FROM zone_quotas quota
JOIN zones zone ON zone.id = quota.zone_id AND zone.deleted_at IS NULL
WHERE quota.zone_id = $1::uuid
`, zoneID)
	quota, err := scanZoneQuota(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return tenancydomain.ZoneQuota{}, tenancydomain.ErrZoneNotFound
	}
	return quota, err
}

func getZoneUsage(
	ctx context.Context,
	query interface {
		QueryRow(context.Context, string, ...any) pgx.Row
	},
	zoneID string,
) (tenancydomain.ZoneUsage, error) {
	var usage tenancydomain.ZoneUsage
	err := query.QueryRow(ctx, `
SELECT
    (SELECT count(*) FROM workspaces
     WHERE zone_id = $1::uuid AND deleted_at IS NULL),
    (SELECT count(DISTINCT member.user_id)
     FROM workspace_members member
     JOIN workspaces workspace ON workspace.id = member.workspace_id
     WHERE workspace.zone_id = $1::uuid
       AND workspace.deleted_at IS NULL
       AND member.status IN ('active', 'invited')),
    (
        COALESCE((
            SELECT sum(file.byte_size)
            FROM files file
            JOIN workspaces workspace ON workspace.id = file.workspace_id
            WHERE workspace.zone_id = $1::uuid
              AND workspace.deleted_at IS NULL
              AND file.deleted_at IS NULL
              AND file.status <> 'deleted'
        ), 0)
        +
        COALESCE((
            SELECT sum(version.byte_size)
            FROM file_versions version
            JOIN files file ON file.id = version.file_id
            JOIN workspaces workspace ON workspace.id = file.workspace_id
            WHERE workspace.zone_id = $1::uuid
              AND workspace.deleted_at IS NULL
              AND file.deleted_at IS NULL
        ), 0)
    ),
    (SELECT count(*) FROM automation_installations
     WHERE zone_id = $1::uuid AND deleted_at IS NULL),
    (
        SELECT count(*)
        FROM (
            SELECT incoming.id
            FROM incoming_webhooks incoming
            JOIN workspaces workspace ON workspace.id = incoming.workspace_id
            WHERE workspace.zone_id = $1::uuid AND workspace.deleted_at IS NULL
            UNION ALL
            SELECT outgoing.id
            FROM outgoing_webhooks outgoing
            JOIN workspaces workspace ON workspace.id = outgoing.workspace_id
            WHERE workspace.zone_id = $1::uuid AND workspace.deleted_at IS NULL
        ) webhook
    )
`, zoneID).Scan(
		&usage.Workspaces,
		&usage.Members,
		&usage.StorageBytes,
		&usage.AutomationInstallations,
		&usage.Webhooks,
	)
	return usage, err
}

func scanZoneQuota(row rowScanner) (tenancydomain.ZoneQuota, error) {
	var quota tenancydomain.ZoneQuota
	err := row.Scan(
		&quota.ZoneID,
		&quota.MaxWorkspaces,
		&quota.MaxMembers,
		&quota.MaxStorageBytes,
		&quota.MaxAutomationInstallations,
		&quota.MaxWebhooks,
		&quota.EnforcementMode,
		&quota.CreatedAt,
		&quota.UpdatedAt,
	)
	return quota, err
}

func scanOIDCProvider(row rowScanner) (tenancydomain.OIDCProvider, error) {
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
	provider.ClientSecretRef = nullStringPtr(clientSecretRef)
	provider.CreatedBy = nullStringPtr(createdBy)
	provider.ClaimMapping = decodeMap(claimMapping)
	return provider, nil
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func boolValue(value *bool) bool {
	return value != nil && *value
}
