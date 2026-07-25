package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	tenancyapp "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/application"
	tenancydomain "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool               *pgxpool.Pool
	defaultWorkspaceID string
}

func NewRepository(pool *pgxpool.Pool, defaultWorkspaceID ...string) *Repository {
	workspaceID := ""
	if len(defaultWorkspaceID) > 0 {
		workspaceID = strings.TrimSpace(defaultWorkspaceID[0])
	}
	return &Repository{pool: pool, defaultWorkspaceID: workspaceID}
}

func (r *Repository) ResolveByDomain(ctx context.Context, domain string) (tenancydomain.ResolvedZone, error) {
	row := r.pool.QueryRow(ctx, resolvedZoneQuery, domain)
	resolved, err := scanResolvedZone(row)
	if err == nil {
		return resolved, nil
	}
	if !errors.Is(err, tenancydomain.ErrZoneNotFound) ||
		(domain != "localhost" && net.ParseIP(domain) == nil) {
		return tenancydomain.ResolvedZone{}, err
	}

	localDomain, err := r.resolveLocalDevelopmentDomain(ctx)
	if err != nil {
		return tenancydomain.ResolvedZone{}, err
	}
	return scanResolvedZone(r.pool.QueryRow(ctx, resolvedZoneQuery, localDomain))
}

func (r *Repository) resolveLocalDevelopmentDomain(ctx context.Context) (string, error) {
	query := `
SELECT zone.kind, zone_domain.domain::text
FROM zones zone
JOIN workspaces workspace
  ON workspace.id = zone.primary_workspace_id
 AND workspace.zone_id = zone.id
 AND workspace.status = 'active'
 AND workspace.deleted_at IS NULL
JOIN LATERAL (
    SELECT domain.domain
    FROM zone_domains domain
    WHERE domain.zone_id = zone.id
      AND domain.status IN ('verified', 'active')
      AND domain.deleted_at IS NULL
    ORDER BY
        CASE domain.kind WHEN 'primary' THEN 0 ELSE 1 END,
        CASE domain.status WHEN 'active' THEN 0 ELSE 1 END,
        domain.created_at
    LIMIT 1
) zone_domain ON true
WHERE zone.status = 'active'
  AND zone.deleted_at IS NULL
`
	args := []any{}
	if r.defaultWorkspaceID != "" {
		query += " AND workspace.id = $1::uuid"
		args = append(args, r.defaultWorkspaceID)
	}
	query += `
 ORDER BY
    CASE zone.kind WHEN 'vpsttt_internal' THEN 0 ELSE 1 END,
    zone.created_at,
    zone.id
 LIMIT 2`

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	type localCandidate struct {
		kind   string
		domain string
	}
	candidates := make([]localCandidate, 0, 2)
	for rows.Next() {
		var candidate localCandidate
		if err := rows.Scan(&candidate.kind, &candidate.domain); err != nil {
			return "", err
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if r.defaultWorkspaceID == "" {
		internalDomains := make([]string, 0, 1)
		for _, candidate := range candidates {
			if candidate.kind == "vpsttt_internal" {
				internalDomains = append(internalDomains, candidate.domain)
			}
		}
		if len(internalDomains) == 1 {
			return internalDomains[0], nil
		}
	}
	if len(candidates) != 1 {
		return "", tenancydomain.ErrZoneNotFound
	}
	return candidates[0].domain, nil
}

func (r *Repository) WorkspaceBelongsToZone(ctx context.Context, workspaceID string, zoneID string) (bool, error) {
	var matches bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM workspaces
    WHERE id = $1::uuid
      AND zone_id = $2::uuid
      AND status = 'active'
      AND deleted_at IS NULL
)
`, workspaceID, zoneID).Scan(&matches)
	return matches, err
}

func (r *Repository) ZoneDomainBelongsToActiveZone(
	ctx context.Context,
	zoneID string,
	domain string,
) (bool, error) {
	var matches bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM zone_domains zone_domain
    JOIN zones zone ON zone.id = zone_domain.zone_id
    WHERE zone_domain.zone_id = $1::uuid
      AND zone_domain.domain = $2
      AND zone_domain.status = 'active'
      AND zone_domain.deleted_at IS NULL
      AND zone.status = 'active'
      AND zone.deleted_at IS NULL
)
`, zoneID, domain).Scan(&matches)
	return matches, err
}

func (r *Repository) ZoneDomainBelongsToRecoverableZone(
	ctx context.Context,
	zoneID string,
	domain string,
) (bool, error) {
	var matches bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM zone_domains zone_domain
    JOIN zones zone ON zone.id = zone_domain.zone_id
    WHERE zone_domain.zone_id = $1::uuid
      AND zone_domain.domain = $2
      AND zone_domain.status = 'active'
      AND zone_domain.deleted_at IS NULL
      AND zone.status IN ('active', 'suspended')
      AND zone.deleted_at IS NULL
)
`, zoneID, domain).Scan(&matches)
	return matches, err
}

func (r *Repository) CreateDomainClaim(ctx context.Context, params tenancyapp.CreateDomainClaimParams) (tenancydomain.DomainClaim, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return tenancydomain.DomainClaim{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	existing, err := findDomainClaimByDomain(ctx, tx, params.Domain)
	if err == nil {
		var ownerMatches bool
		if err := tx.QueryRow(ctx, `
SELECT created_by = $2::uuid
FROM zones
WHERE id = $1::uuid AND deleted_at IS NULL
`, existing.Zone.ID, params.ActorUserID).Scan(&ownerMatches); err != nil {
			return tenancydomain.DomainClaim{}, err
		}
		if !ownerMatches {
			return tenancydomain.DomainClaim{}, tenancydomain.ErrDomainAlreadyClaimed
		}
		if err := tx.Commit(ctx); err != nil {
			return tenancydomain.DomainClaim{}, err
		}
		return existing, nil
	}
	if !errors.Is(err, tenancydomain.ErrDomainClaimNotFound) {
		return tenancydomain.DomainClaim{}, err
	}

	metadata := map[string]any{
		"template_key": "customer_standard",
		"capabilities": map[string]any{
			"chat": true, "files": true, "calls": true, "bots": true,
			"automation": true, "webhooks": true, "federation": false, "sso": false,
		},
	}
	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return tenancydomain.DomainClaim{}, err
	}

	var claim tenancydomain.DomainClaim
	var primaryWorkspaceID sql.NullString
	var zoneMetadata []byte
	err = tx.QueryRow(ctx, `
INSERT INTO zones (
    slug, name, kind, status, registration_mode, created_by, metadata
)
VALUES (
    $1, $2, 'customer_saas', 'provisioning', $3, $4::uuid, $5::jsonb
)
RETURNING id::text, slug::text, name, kind, status, registration_mode,
          primary_workspace_id::text, metadata, created_at, updated_at
`, params.ZoneSlug, params.ZoneName, params.RegistrationMode, params.ActorUserID, string(metadataJSON)).Scan(
		&claim.Zone.ID,
		&claim.Zone.Slug,
		&claim.Zone.Name,
		&claim.Zone.Kind,
		&claim.Zone.Status,
		&claim.Zone.RegistrationMode,
		&primaryWorkspaceID,
		&zoneMetadata,
		&claim.Zone.CreatedAt,
		&claim.Zone.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return tenancydomain.DomainClaim{}, tenancydomain.ErrDomainAlreadyClaimed
		}
		return tenancydomain.DomainClaim{}, err
	}
	claim.Zone.PrimaryWorkspaceID = nullStringPtr(primaryWorkspaceID)
	claim.Zone.Metadata = decodeMap(zoneMetadata)

	var verifiedAt sql.NullTime
	var verificationExpiresAt sql.NullTime
	var lastVerificationError sql.NullString
	var lastCheckedAt sql.NullTime
	err = tx.QueryRow(ctx, `
INSERT INTO zone_domains (
    zone_id, domain, kind, status, verification_method,
    verification_expires_at, tls_status, created_by,
    metadata
)
VALUES (
    $1::uuid, $2, 'primary', 'pending', 'dns_txt',
    $3, 'pending', $4::uuid,
    '{"claim_source":"self_service"}'::jsonb
)
RETURNING id::text, zone_id::text, domain::text, kind, status,
          verification_method, verification_token, verified_at,
          verification_expires_at, verification_attempts,
          last_verification_error, tls_status, last_checked_at,
          created_at, updated_at
`, claim.Zone.ID, params.Domain, params.ExpiresAt, params.ActorUserID).Scan(
		&claim.Domain.ID,
		&claim.Domain.ZoneID,
		&claim.Domain.Domain,
		&claim.Domain.Kind,
		&claim.Domain.Status,
		&claim.Domain.VerificationMethod,
		&claim.Domain.VerificationToken,
		&verifiedAt,
		&verificationExpiresAt,
		&claim.Domain.VerificationAttempts,
		&lastVerificationError,
		&claim.Domain.TLSStatus,
		&lastCheckedAt,
		&claim.Domain.CreatedAt,
		&claim.Domain.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return tenancydomain.DomainClaim{}, tenancydomain.ErrDomainAlreadyClaimed
		}
		return tenancydomain.DomainClaim{}, err
	}
	claim.Domain.VerifiedAt = nullTimePtr(verifiedAt)
	claim.Domain.VerificationExpiresAt = nullTimePtr(verificationExpiresAt)
	claim.Domain.LastVerificationError = nullStringPtr(lastVerificationError)
	claim.Domain.LastCheckedAt = nullTimePtr(lastCheckedAt)

	if err := tx.Commit(ctx); err != nil {
		return tenancydomain.DomainClaim{}, err
	}
	return claim, nil
}

func (r *Repository) FindDomainClaim(ctx context.Context, domainID string, actorUserID string) (tenancydomain.DomainClaim, error) {
	return findDomainClaim(ctx, r.pool, domainID, actorUserID, false)
}

func (r *Repository) RecordDomainVerificationFailure(
	ctx context.Context,
	domainID string,
	actorUserID string,
	checkedAt time.Time,
	reason string,
) error {
	command, err := r.pool.Exec(ctx, `
UPDATE zone_domains domain
SET verification_attempts = verification_attempts + 1,
    last_checked_at = $3,
    last_verification_error = $4
FROM zones zone
WHERE domain.id = $1::uuid
  AND domain.zone_id = zone.id
  AND (
      domain.created_by = $2::uuid
      OR zone.created_by = $2::uuid
      OR EXISTS (
          SELECT 1
          FROM workspaces workspace
          JOIN workspace_member_roles member_role
            ON member_role.workspace_id = workspace.id
           AND member_role.user_id = $2::uuid
          JOIN roles role
            ON role.id = member_role.role_id
           AND role.deleted_at IS NULL
          WHERE workspace.zone_id = zone.id
            AND workspace.deleted_at IS NULL
            AND role.code IN ('workspace_owner', 'workspace_admin')
      )
  )
  AND domain.status = 'pending'
  AND domain.deleted_at IS NULL
  AND zone.deleted_at IS NULL
`, domainID, actorUserID, checkedAt, reason)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return tenancydomain.ErrDomainClaimNotFound
	}
	return nil
}

func (r *Repository) ProvisionVerifiedDomain(
	ctx context.Context,
	params tenancyapp.ProvisionVerifiedDomainParams,
) (tenancydomain.ResolvedZone, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return tenancydomain.ResolvedZone{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	claim, err := findDomainClaim(ctx, tx, params.DomainID, params.ActorUserID, true)
	if err != nil {
		return tenancydomain.ResolvedZone{}, err
	}
	if claim.Domain.Status == "active" || claim.Domain.Status == "verified" {
		if err := tx.Commit(ctx); err != nil {
			return tenancydomain.ResolvedZone{}, err
		}
		return r.ResolveByDomain(ctx, claim.Domain.Domain)
	}
	if claim.Domain.Status != "pending" {
		return tenancydomain.ResolvedZone{}, tenancydomain.ErrDomainClaimNotFound
	}
	if claim.Domain.VerificationExpiresAt == nil || !claim.Domain.VerificationExpiresAt.After(params.VerifiedAt) {
		return tenancydomain.ResolvedZone{}, tenancydomain.ErrDomainVerificationExpired
	}

	workspaceID, err := provisionCustomerWorkspace(ctx, tx, claim, params.ActorUserID)
	if err != nil {
		return tenancydomain.ResolvedZone{}, err
	}

	if _, err := tx.Exec(ctx, `
UPDATE zones
SET primary_workspace_id = $2::uuid,
    status = 'active'
WHERE id = $1::uuid
  AND status = 'provisioning'
  AND deleted_at IS NULL
`, claim.Zone.ID, workspaceID); err != nil {
		return tenancydomain.ResolvedZone{}, err
	}
	if _, err := tx.Exec(ctx, `
UPDATE zone_domains
SET status = 'active',
    verified_at = $2,
    last_checked_at = $2,
    last_verification_error = NULL
WHERE id = $1::uuid
  AND status = 'pending'
  AND deleted_at IS NULL
`, claim.Domain.ID, params.VerifiedAt); err != nil {
		return tenancydomain.ResolvedZone{}, err
	}

	baseURL := "https://" + claim.Domain.Domain
	if _, err := tx.Exec(ctx, `
INSERT INTO zone_deployments (
    zone_id, mode, web_base_url, api_base_url, ws_base_url, admin_base_url,
    database_mode, storage_bucket, redis_prefix, status, metadata
)
SELECT
    $1::uuid,
    'shared',
    $2,
    $2,
    $3,
    $4,
    'shared_schema',
    $5,
    $6,
    'ready',
    '{"provisioner":"vpsttt_control_plane","tls":"caddy_on_demand"}'::jsonb
WHERE NOT EXISTS (
    SELECT 1
    FROM zone_deployments
    WHERE zone_id = $1::uuid AND deleted_at IS NULL
)
`, claim.Zone.ID, baseURL, "wss://"+claim.Domain.Domain+"/ws", baseURL+"/admin",
		zoneStorageBucket(claim.Zone.ID), "zone:"+claim.Zone.ID); err != nil {
		return tenancydomain.ResolvedZone{}, err
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (
    zone_id, workspace_id, actor_user_id, action, entity_type, entity_id, metadata
)
VALUES (
    $1::uuid, $2::uuid, $3::uuid, 'zone.domain_verified',
    'zone_domain', $4::uuid,
    jsonb_build_object('domain', $5, 'verification_method', 'dns_txt')
)
`, claim.Zone.ID, workspaceID, params.ActorUserID, claim.Domain.ID, claim.Domain.Domain); err != nil {
		return tenancydomain.ResolvedZone{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return tenancydomain.ResolvedZone{}, err
	}
	return r.ResolveByDomain(ctx, claim.Domain.Domain)
}

func (r *Repository) CanManageZone(ctx context.Context, zoneID string, userID string) (bool, error) {
	var allowed bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM zones zone
    WHERE zone.id = $1::uuid
      AND zone.deleted_at IS NULL
      AND (
          zone.created_by = $2::uuid
          OR EXISTS (
              SELECT 1
              FROM workspaces workspace
              JOIN workspace_member_roles member_role
                ON member_role.workspace_id = workspace.id
               AND member_role.user_id = $2::uuid
              JOIN roles role
                ON role.id = member_role.role_id
               AND role.deleted_at IS NULL
              WHERE workspace.zone_id = zone.id
                AND workspace.deleted_at IS NULL
                AND role.code IN ('workspace_owner', 'workspace_admin')
          )
      )
)
`, zoneID, userID).Scan(&allowed)
	return allowed, err
}

func (r *Repository) ListAutomationTemplates(ctx context.Context, zoneID string) ([]tenancydomain.AutomationTemplate, error) {
	rows, err := r.pool.Query(ctx, `
SELECT
    template.id::text,
    template.key::text,
    template.name,
    template.description,
    template.zone_kind,
    template.template_type,
    template.runtime_kind,
    template.config_schema,
    template.default_config,
    template.required_scopes,
    template.status,
    template.created_at,
    template.updated_at
FROM automation_templates template
JOIN zones zone
  ON zone.id = $1::uuid
 AND zone.deleted_at IS NULL
WHERE template.deleted_at IS NULL
  AND template.status = 'active'
  AND template.zone_kind IN ('any', zone.kind)
ORDER BY template.name, template.key
`, zoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	templates := make([]tenancydomain.AutomationTemplate, 0)
	for rows.Next() {
		template, err := scanAutomationTemplate(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, template)
	}
	return templates, rows.Err()
}

func (r *Repository) GetAutomationTemplate(
	ctx context.Context,
	zoneID string,
	templateKey string,
) (tenancydomain.AutomationTemplate, error) {
	row := r.pool.QueryRow(ctx, `
SELECT
    template.id::text,
    template.key::text,
    template.name,
    template.description,
    template.zone_kind,
    template.template_type,
    template.runtime_kind,
    template.config_schema,
    template.default_config,
    template.required_scopes,
    template.status,
    template.created_at,
    template.updated_at
FROM automation_templates template
JOIN zones zone
  ON zone.id = $1::uuid
 AND zone.status = 'active'
 AND zone.deleted_at IS NULL
WHERE template.key = $2
  AND template.status = 'active'
  AND template.deleted_at IS NULL
  AND template.zone_kind IN ('any', zone.kind)
LIMIT 1
`, zoneID, templateKey)
	template, err := scanAutomationTemplate(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return tenancydomain.AutomationTemplate{}, tenancydomain.ErrAutomationTemplateNotFound
	}
	return template, err
}

func (r *Repository) ListAutomationInstallations(ctx context.Context, zoneID string) ([]tenancydomain.AutomationInstallation, error) {
	rows, err := r.pool.Query(ctx, `
SELECT
    installation.id::text,
    installation.zone_id::text,
    installation.workspace_id::text,
    installation.template_id::text,
    template.key::text,
    installation.name,
    installation.status,
    installation.config,
    installation.secret_ref,
    installation.runtime_webhook_id::text,
    installation.installed_by::text,
    installation.created_at,
    installation.updated_at
FROM automation_installations installation
LEFT JOIN automation_templates template
  ON template.id = installation.template_id
 AND template.deleted_at IS NULL
WHERE installation.zone_id = $1::uuid
  AND installation.deleted_at IS NULL
ORDER BY installation.created_at DESC
`, zoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	installations := make([]tenancydomain.AutomationInstallation, 0)
	for rows.Next() {
		installation, err := scanAutomationInstallation(rows)
		if err != nil {
			return nil, err
		}
		installations = append(installations, installation)
	}
	return installations, rows.Err()
}

func (r *Repository) CreateAutomationInstallation(
	ctx context.Context,
	params tenancyapp.CreateAutomationInstallationParams,
) (tenancydomain.AutomationInstallation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return tenancydomain.AutomationInstallation{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	template, err := findAutomationTemplate(ctx, tx, params.ZoneID, params.TemplateKey)
	if err != nil {
		return tenancydomain.AutomationInstallation{}, err
	}
	if err := ensureAutomationQuota(ctx, tx, params.ZoneID); err != nil {
		return tenancydomain.AutomationInstallation{}, err
	}
	workspaceID := strings.TrimSpace(params.WorkspaceID)
	if workspaceID == "" {
		if err := tx.QueryRow(ctx, `
SELECT primary_workspace_id::text
FROM zones
WHERE id = $1::uuid
  AND status = 'active'
  AND deleted_at IS NULL
`, params.ZoneID).Scan(&workspaceID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return tenancydomain.AutomationInstallation{}, tenancydomain.ErrZoneNotFound
			}
			return tenancydomain.AutomationInstallation{}, err
		}
	}
	var workspaceMatches bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM workspaces
    WHERE id = $1::uuid
      AND zone_id = $2::uuid
      AND status = 'active'
      AND deleted_at IS NULL
)
`, workspaceID, params.ZoneID).Scan(&workspaceMatches); err != nil {
		return tenancydomain.AutomationInstallation{}, err
	}
	if !workspaceMatches {
		return tenancydomain.AutomationInstallation{}, tenancydomain.ErrWorkspaceZoneMismatch
	}

	config := mergeConfig(template.DefaultConfig, params.Config)
	if err := tenancyapp.ValidateAutomationConfig(template.ConfigSchema, config); err != nil {
		return tenancydomain.AutomationInstallation{}, fmt.Errorf(
			"%w: %v",
			tenancydomain.ErrAutomationConfigInvalid,
			err,
		)
	}
	configJSON, err := json.Marshal(config)
	if err != nil {
		return tenancydomain.AutomationInstallation{}, err
	}

	runtimeWebhookID := ""
	if template.RuntimeKind == "outgoing_webhook" {
		targetURL, eventTypes, err := automationRuntimeValues(config)
		if err != nil {
			return tenancydomain.AutomationInstallation{}, err
		}
		if strings.TrimSpace(params.RuntimeSecretEncrypted) == "" {
			return tenancydomain.AutomationInstallation{}, fmt.Errorf(
				"%w: outgoing webhook runtime is incomplete",
				tenancydomain.ErrAutomationConfigInvalid,
			)
		}
		if err := validateAutomationChannel(ctx, tx, workspaceID, config); err != nil {
			return tenancydomain.AutomationInstallation{}, err
		}
		runtimeStatus := "disabled"
		if params.Status == "enabled" {
			runtimeStatus = "active"
		}
		if err := tx.QueryRow(ctx, `
INSERT INTO outgoing_webhooks (
    workspace_id, name, target_url, signing_secret_encrypted, event_types, status, created_by
)
VALUES ($1::uuid, $2, $3, $4, $5::text[], $6, $7::uuid)
RETURNING id::text
`, workspaceID, params.Name, targetURL, params.RuntimeSecretEncrypted,
			eventTypes, runtimeStatus, params.ActorUserID).Scan(&runtimeWebhookID); err != nil {
			return tenancydomain.AutomationInstallation{}, err
		}
	}

	row := tx.QueryRow(ctx, `
INSERT INTO automation_installations (
    zone_id, workspace_id, template_id, name, status, config, secret_ref,
    installed_by, runtime_webhook_id
)
VALUES (
    $1::uuid, $2::uuid, $3::uuid, $4, $5, $6::jsonb, NULLIF($7, ''),
    $8::uuid, NULLIF($9, '')::uuid
)
RETURNING
    id::text,
    zone_id::text,
    workspace_id::text,
    template_id::text,
    $10::text,
    name,
    status,
    config,
    secret_ref,
    runtime_webhook_id::text,
    installed_by::text,
    created_at,
    updated_at
`, params.ZoneID, workspaceID, template.ID, params.Name, params.Status, string(configJSON),
		params.SecretRef, params.ActorUserID, runtimeWebhookID, template.Key)
	installation, err := scanAutomationInstallation(row)
	if err != nil {
		if isUniqueViolation(err) {
			return tenancydomain.AutomationInstallation{}, tenancydomain.ErrAutomationInstallationConflict
		}
		return tenancydomain.AutomationInstallation{}, err
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (
    zone_id, workspace_id, actor_user_id, action, entity_type, entity_id, metadata
)
VALUES (
    $1::uuid, $2::uuid, $3::uuid, 'automation.install',
    'automation_installation', $4::uuid,
    jsonb_build_object('template_key', $5, 'status', $6)
)
`, params.ZoneID, workspaceID, params.ActorUserID, installation.ID, template.Key, params.Status); err != nil {
		return tenancydomain.AutomationInstallation{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return tenancydomain.AutomationInstallation{}, err
	}
	return installation, nil
}

func (r *Repository) UpdateAutomationInstallation(
	ctx context.Context,
	params tenancyapp.UpdateAutomationInstallationParams,
) (tenancydomain.AutomationInstallation, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return tenancydomain.AutomationInstallation{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var workspaceID string
	var templateKey string
	var runtimeKind string
	var runtimeWebhookID sql.NullString
	var currentConfigJSON []byte
	var defaultConfigJSON []byte
	var configSchemaJSON []byte
	if err := tx.QueryRow(ctx, `
SELECT
    installation.workspace_id::text,
    COALESCE(template.key::text, ''),
    COALESCE(template.runtime_kind, 'none'),
    installation.runtime_webhook_id::text,
    installation.config,
    COALESCE(template.default_config, '{}'::jsonb),
    COALESCE(template.config_schema, '{}'::jsonb)
FROM automation_installations installation
LEFT JOIN automation_templates template
  ON template.id = installation.template_id
 AND template.deleted_at IS NULL
WHERE installation.id = $1::uuid
  AND installation.zone_id = $2::uuid
  AND installation.deleted_at IS NULL
FOR UPDATE OF installation
`, params.InstallationID, params.ZoneID).Scan(
		&workspaceID,
		&templateKey,
		&runtimeKind,
		&runtimeWebhookID,
		&currentConfigJSON,
		&defaultConfigJSON,
		&configSchemaJSON,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tenancydomain.AutomationInstallation{}, tenancydomain.ErrAutomationInstallationNotFound
		}
		return tenancydomain.AutomationInstallation{}, err
	}

	configJSON := currentConfigJSON
	if params.Config != nil {
		if templateKey == "" {
			return tenancydomain.AutomationInstallation{}, tenancydomain.ErrAutomationTemplateNotFound
		}
		config := mergeConfig(decodeMap(defaultConfigJSON), *params.Config)
		if err := tenancyapp.ValidateAutomationConfig(decodeMap(configSchemaJSON), config); err != nil {
			return tenancydomain.AutomationInstallation{}, fmt.Errorf(
				"%w: %v",
				tenancydomain.ErrAutomationConfigInvalid,
				err,
			)
		}
		configJSON, err = json.Marshal(config)
		if err != nil {
			return tenancydomain.AutomationInstallation{}, err
		}
	}

	name := ""
	if params.Name != nil {
		name = *params.Name
	}
	status := ""
	if params.Status != nil {
		status = *params.Status
	}
	secretRef := ""
	if params.SecretRef != nil {
		secretRef = *params.SecretRef
	}

	row := tx.QueryRow(ctx, `
WITH updated AS (
    UPDATE automation_installations
    SET name = CASE WHEN $3::boolean THEN $4 ELSE name END,
        status = CASE WHEN $5::boolean THEN $6 ELSE status END,
        config = CASE WHEN $7::boolean THEN $8::jsonb ELSE config END,
        secret_ref = CASE WHEN $9::boolean THEN NULLIF($10, '') ELSE secret_ref END
    WHERE id = $1::uuid
      AND zone_id = $2::uuid
      AND deleted_at IS NULL
    RETURNING *
)
SELECT
    updated.id::text,
    updated.zone_id::text,
    updated.workspace_id::text,
    updated.template_id::text,
    template.key::text,
    updated.name,
    updated.status,
    updated.config,
    updated.secret_ref,
    updated.runtime_webhook_id::text,
    updated.installed_by::text,
    updated.created_at,
    updated.updated_at
FROM updated
LEFT JOIN automation_templates template ON template.id = updated.template_id
`, params.InstallationID, params.ZoneID,
		params.Name != nil, name,
		params.Status != nil, status,
		params.Config != nil, string(configJSON),
		params.SecretRef != nil, secretRef)
	installation, err := scanAutomationInstallation(row)
	if err != nil {
		if isUniqueViolation(err) {
			return tenancydomain.AutomationInstallation{}, tenancydomain.ErrAutomationInstallationConflict
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return tenancydomain.AutomationInstallation{}, tenancydomain.ErrAutomationInstallationNotFound
		}
		return tenancydomain.AutomationInstallation{}, err
	}

	if runtimeKind == "outgoing_webhook" {
		if !runtimeWebhookID.Valid {
			return tenancydomain.AutomationInstallation{}, fmt.Errorf(
				"%w: runtime webhook is missing; recreate this installation",
				tenancydomain.ErrAutomationConfigInvalid,
			)
		}
		targetURL, eventTypes, err := automationRuntimeValues(installation.Config)
		if err != nil {
			return tenancydomain.AutomationInstallation{}, err
		}
		if err := validateAutomationChannel(ctx, tx, workspaceID, installation.Config); err != nil {
			return tenancydomain.AutomationInstallation{}, err
		}
		runtimeStatus := "disabled"
		if installation.Status == "enabled" {
			runtimeStatus = "active"
		}
		tag, err := tx.Exec(ctx, `
UPDATE outgoing_webhooks
SET name = $3,
    target_url = $4,
    event_types = $5::text[],
    status = $6
WHERE id = $1::uuid
  AND workspace_id = $2::uuid
`, runtimeWebhookID.String, workspaceID, installation.Name, targetURL, eventTypes, runtimeStatus)
		if err != nil {
			return tenancydomain.AutomationInstallation{}, err
		}
		if tag.RowsAffected() == 0 {
			return tenancydomain.AutomationInstallation{}, fmt.Errorf(
				"%w: runtime webhook is unavailable",
				tenancydomain.ErrAutomationConfigInvalid,
			)
		}
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (
    zone_id, workspace_id, actor_user_id, action, entity_type, entity_id, metadata
)
VALUES (
    $1::uuid, $2::uuid, $3::uuid, 'automation.update',
    'automation_installation', $4::uuid,
    jsonb_build_object(
        'name_changed', $5::boolean,
        'status_changed', $6::boolean,
        'config_changed', $7::boolean,
        'secret_ref_changed', $8::boolean
    )
)
`, params.ZoneID, workspaceID, params.ActorUserID, params.InstallationID,
		params.Name != nil, params.Status != nil, params.Config != nil, params.SecretRef != nil); err != nil {
		return tenancydomain.AutomationInstallation{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return tenancydomain.AutomationInstallation{}, err
	}
	return installation, nil
}

func (r *Repository) DeleteAutomationInstallation(
	ctx context.Context,
	zoneID string,
	installationID string,
	actorUserID string,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var workspaceID string
	var runtimeWebhookID sql.NullString
	if err := tx.QueryRow(ctx, `
UPDATE automation_installations
SET status = 'disabled',
    deleted_at = now()
WHERE id = $1::uuid
  AND zone_id = $2::uuid
  AND deleted_at IS NULL
RETURNING workspace_id::text, runtime_webhook_id::text
`, installationID, zoneID).Scan(&workspaceID, &runtimeWebhookID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tenancydomain.ErrAutomationInstallationNotFound
		}
		return err
	}

	if runtimeWebhookID.Valid {
		if _, err := tx.Exec(ctx, `
DELETE FROM outgoing_webhooks
WHERE id = $1::uuid
  AND workspace_id = $2::uuid
`, runtimeWebhookID.String, workspaceID); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (
    zone_id, workspace_id, actor_user_id, action, entity_type, entity_id
)
VALUES (
    $1::uuid, $2::uuid, $3::uuid, 'automation.uninstall',
    'automation_installation', $4::uuid
)
`, zoneID, workspaceID, actorUserID, installationID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

const resolvedZoneQuery = `
SELECT
    z.id::text,
    z.slug::text,
    z.name,
    z.kind,
    z.status,
    z.registration_mode,
    z.primary_workspace_id::text,
    z.metadata,
    z.created_at,
    z.updated_at,
    zd.id::text,
    zd.zone_id::text,
    zd.domain::text,
    zd.kind,
    zd.status,
    zd.verification_method,
    zd.verification_token,
    zd.verified_at,
    zd.verification_expires_at,
    zd.verification_attempts,
    zd.last_verification_error,
    zd.tls_status,
    zd.last_checked_at,
    zd.created_at,
    zd.updated_at,
    deployment.id::text,
    deployment.zone_id::text,
    deployment.mode,
    deployment.web_base_url,
    deployment.api_base_url,
    deployment.ws_base_url,
    deployment.admin_base_url,
    deployment.database_mode,
    deployment.storage_bucket,
    deployment.redis_prefix,
    deployment.status,
    deployment.metadata,
    workspace.id::text,
    workspace.slug::text,
    workspace.name
FROM zone_domains zd
JOIN zones z
  ON z.id = zd.zone_id
 AND z.deleted_at IS NULL
LEFT JOIN LATERAL (
    SELECT *
    FROM zone_deployments d
    WHERE d.zone_id = z.id
      AND d.deleted_at IS NULL
    ORDER BY
        CASE d.status
            WHEN 'ready' THEN 0
            WHEN 'provisioning' THEN 1
            ELSE 2
        END,
        d.created_at DESC
    LIMIT 1
) deployment ON true
LEFT JOIN workspaces workspace
  ON workspace.id = z.primary_workspace_id
 AND workspace.zone_id = z.id
 AND workspace.deleted_at IS NULL
WHERE zd.domain = $1
  AND zd.deleted_at IS NULL
  AND zd.status IN ('verified', 'active')
  AND z.status IN ('active', 'suspended')
ORDER BY
    CASE zd.status WHEN 'active' THEN 0 ELSE 1 END,
    zd.created_at DESC
LIMIT 1
`

type rowScanner interface {
	Scan(dest ...any) error
}

type queryer interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func scanResolvedZone(row rowScanner) (tenancydomain.ResolvedZone, error) {
	var resolved tenancydomain.ResolvedZone
	var primaryWorkspaceID sql.NullString
	var zoneMetadata []byte
	var verifiedAt sql.NullTime
	var verificationExpiresAt sql.NullTime
	var lastVerificationError sql.NullString
	var lastCheckedAt sql.NullTime
	var deploymentID sql.NullString
	var deploymentZoneID sql.NullString
	var deploymentMode sql.NullString
	var webBaseURL sql.NullString
	var apiBaseURL sql.NullString
	var wsBaseURL sql.NullString
	var adminBaseURL sql.NullString
	var databaseMode sql.NullString
	var storageBucket sql.NullString
	var redisPrefix sql.NullString
	var deploymentStatus sql.NullString
	var deploymentMetadata []byte
	var workspaceID sql.NullString
	var workspaceSlug sql.NullString
	var workspaceName sql.NullString

	if err := row.Scan(
		&resolved.Zone.ID,
		&resolved.Zone.Slug,
		&resolved.Zone.Name,
		&resolved.Zone.Kind,
		&resolved.Zone.Status,
		&resolved.Zone.RegistrationMode,
		&primaryWorkspaceID,
		&zoneMetadata,
		&resolved.Zone.CreatedAt,
		&resolved.Zone.UpdatedAt,
		&resolved.Domain.ID,
		&resolved.Domain.ZoneID,
		&resolved.Domain.Domain,
		&resolved.Domain.Kind,
		&resolved.Domain.Status,
		&resolved.Domain.VerificationMethod,
		&resolved.Domain.VerificationToken,
		&verifiedAt,
		&verificationExpiresAt,
		&resolved.Domain.VerificationAttempts,
		&lastVerificationError,
		&resolved.Domain.TLSStatus,
		&lastCheckedAt,
		&resolved.Domain.CreatedAt,
		&resolved.Domain.UpdatedAt,
		&deploymentID,
		&deploymentZoneID,
		&deploymentMode,
		&webBaseURL,
		&apiBaseURL,
		&wsBaseURL,
		&adminBaseURL,
		&databaseMode,
		&storageBucket,
		&redisPrefix,
		&deploymentStatus,
		&deploymentMetadata,
		&workspaceID,
		&workspaceSlug,
		&workspaceName,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tenancydomain.ResolvedZone{}, tenancydomain.ErrZoneNotFound
		}
		return tenancydomain.ResolvedZone{}, err
	}

	resolved.Zone.PrimaryWorkspaceID = nullStringPtr(primaryWorkspaceID)
	resolved.Zone.Metadata = decodeMap(zoneMetadata)
	resolved.Domain.VerifiedAt = nullTimePtr(verifiedAt)
	resolved.Domain.VerificationExpiresAt = nullTimePtr(verificationExpiresAt)
	resolved.Domain.LastVerificationError = nullStringPtr(lastVerificationError)
	resolved.Domain.LastCheckedAt = nullTimePtr(lastCheckedAt)

	if deploymentID.Valid {
		resolved.Deployment = &tenancydomain.Deployment{
			ID:            deploymentID.String,
			ZoneID:        deploymentZoneID.String,
			Mode:          deploymentMode.String,
			WebBaseURL:    webBaseURL.String,
			APIBaseURL:    apiBaseURL.String,
			WSBaseURL:     wsBaseURL.String,
			AdminBaseURL:  nullStringPtr(adminBaseURL),
			DatabaseMode:  databaseMode.String,
			StorageBucket: nullStringPtr(storageBucket),
			RedisPrefix:   nullStringPtr(redisPrefix),
			Status:        deploymentStatus.String,
			Metadata:      decodeMap(deploymentMetadata),
		}
	}

	if workspaceID.Valid {
		resolved.Workspace = &tenancydomain.WorkspaceRef{
			ID:   workspaceID.String,
			Slug: workspaceSlug.String,
			Name: workspaceName.String,
		}
	}
	return resolved, nil
}

func findDomainClaim(
	ctx context.Context,
	db queryer,
	domainID string,
	actorUserID string,
	forUpdate bool,
) (tenancydomain.DomainClaim, error) {
	query := domainClaimSelect + `
WHERE domain.id = $1::uuid
  AND (
      domain.created_by = $2::uuid
      OR zone.created_by = $2::uuid
      OR EXISTS (
          SELECT 1
          FROM workspaces managed_workspace
          JOIN workspace_member_roles member_role
            ON member_role.workspace_id = managed_workspace.id
           AND member_role.user_id = $2::uuid
          JOIN roles role
            ON role.id = member_role.role_id
           AND role.deleted_at IS NULL
          WHERE managed_workspace.zone_id = zone.id
            AND managed_workspace.deleted_at IS NULL
            AND role.code IN ('workspace_owner', 'workspace_admin')
      )
  )
  AND domain.deleted_at IS NULL
  AND zone.deleted_at IS NULL
`
	if forUpdate {
		query += " FOR UPDATE OF domain, zone"
	}
	return scanDomainClaim(db.QueryRow(ctx, query, domainID, actorUserID))
}

func findDomainClaimByDomain(ctx context.Context, db queryer, domain string) (tenancydomain.DomainClaim, error) {
	return scanDomainClaim(db.QueryRow(ctx, domainClaimSelect+`
WHERE domain.domain = $1
  AND domain.deleted_at IS NULL
  AND zone.deleted_at IS NULL
ORDER BY domain.created_at DESC
LIMIT 1
`, domain))
}

const domainClaimSelect = `
SELECT
    zone.id::text,
    zone.slug::text,
    zone.name,
    zone.kind,
    zone.status,
    zone.registration_mode,
    zone.primary_workspace_id::text,
    zone.metadata,
    zone.created_at,
    zone.updated_at,
    domain.id::text,
    domain.zone_id::text,
    domain.domain::text,
    domain.kind,
    domain.status,
    domain.verification_method,
    domain.verification_token,
    domain.verified_at,
    domain.verification_expires_at,
    domain.verification_attempts,
    domain.last_verification_error,
    domain.tls_status,
    domain.last_checked_at,
    domain.created_at,
    domain.updated_at,
    workspace.id::text,
    workspace.slug::text,
    workspace.name
FROM zone_domains domain
JOIN zones zone
  ON zone.id = domain.zone_id
LEFT JOIN workspaces workspace
  ON workspace.id = zone.primary_workspace_id
 AND workspace.zone_id = zone.id
 AND workspace.deleted_at IS NULL
`

func scanDomainClaim(row rowScanner) (tenancydomain.DomainClaim, error) {
	var claim tenancydomain.DomainClaim
	var primaryWorkspaceID sql.NullString
	var zoneMetadata []byte
	var verifiedAt sql.NullTime
	var verificationExpiresAt sql.NullTime
	var lastVerificationError sql.NullString
	var lastCheckedAt sql.NullTime
	var workspaceID sql.NullString
	var workspaceSlug sql.NullString
	var workspaceName sql.NullString
	if err := row.Scan(
		&claim.Zone.ID,
		&claim.Zone.Slug,
		&claim.Zone.Name,
		&claim.Zone.Kind,
		&claim.Zone.Status,
		&claim.Zone.RegistrationMode,
		&primaryWorkspaceID,
		&zoneMetadata,
		&claim.Zone.CreatedAt,
		&claim.Zone.UpdatedAt,
		&claim.Domain.ID,
		&claim.Domain.ZoneID,
		&claim.Domain.Domain,
		&claim.Domain.Kind,
		&claim.Domain.Status,
		&claim.Domain.VerificationMethod,
		&claim.Domain.VerificationToken,
		&verifiedAt,
		&verificationExpiresAt,
		&claim.Domain.VerificationAttempts,
		&lastVerificationError,
		&claim.Domain.TLSStatus,
		&lastCheckedAt,
		&claim.Domain.CreatedAt,
		&claim.Domain.UpdatedAt,
		&workspaceID,
		&workspaceSlug,
		&workspaceName,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tenancydomain.DomainClaim{}, tenancydomain.ErrDomainClaimNotFound
		}
		return tenancydomain.DomainClaim{}, err
	}
	claim.Zone.PrimaryWorkspaceID = nullStringPtr(primaryWorkspaceID)
	claim.Zone.Metadata = decodeMap(zoneMetadata)
	claim.Domain.VerifiedAt = nullTimePtr(verifiedAt)
	claim.Domain.VerificationExpiresAt = nullTimePtr(verificationExpiresAt)
	claim.Domain.LastVerificationError = nullStringPtr(lastVerificationError)
	claim.Domain.LastCheckedAt = nullTimePtr(lastCheckedAt)
	if workspaceID.Valid {
		claim.Workspace = &tenancydomain.WorkspaceRef{
			ID: workspaceID.String, Slug: workspaceSlug.String, Name: workspaceName.String,
		}
	}
	return claim, nil
}

func provisionCustomerWorkspace(
	ctx context.Context,
	tx pgx.Tx,
	claim tenancydomain.DomainClaim,
	ownerID string,
) (string, error) {
	var workspaceID string
	err := tx.QueryRow(ctx, `
INSERT INTO workspaces (zone_id, slug, name, description, owner_id, plan, status)
VALUES (
    $1::uuid, $2, $3, 'Workspace duoc tao tu domain ' || $4,
    $5::uuid, 'team', 'active'
)
RETURNING id::text
`, claim.Zone.ID, claim.Zone.Slug, claim.Zone.Name, claim.Domain.Domain, ownerID).Scan(&workspaceID)
	if err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO workspace_members (workspace_id, user_id, status, joined_at)
VALUES ($1::uuid, $2::uuid, 'active', now())
ON CONFLICT (workspace_id, user_id)
DO UPDATE SET status = 'active', joined_at = COALESCE(workspace_members.joined_at, now())
`, workspaceID, ownerID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO workspace_member_roles (workspace_id, user_id, role_id, assigned_by)
SELECT $1::uuid, $2::uuid, role.id, $2::uuid
FROM roles role
WHERE role.workspace_id IS NULL
  AND role.code = 'workspace_owner'
  AND role.deleted_at IS NULL
ORDER BY role.created_at
LIMIT 1
ON CONFLICT DO NOTHING
`, workspaceID, ownerID); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
WITH definitions(slug, name, description) AS (
    VALUES
        ('general', 'General', 'Trao doi chung trong workspace'),
        ('announcements', 'Announcements', 'Thong bao noi bo cua workspace')
),
inserted AS (
    INSERT INTO channels (
        workspace_id, slug, name, description, type, created_by, settings
    )
    SELECT
        $1::uuid,
        definitions.slug,
        definitions.name,
        definitions.description,
        'public',
        $2::uuid,
        '{"system_default":true,"template_key":"customer_standard"}'::jsonb
    FROM definitions
    RETURNING id
)
INSERT INTO channel_members (channel_id, user_id, status)
SELECT id, $2::uuid, 'active'
FROM inserted
ON CONFLICT (channel_id, user_id) DO NOTHING
`, workspaceID, ownerID); err != nil {
		return "", err
	}
	return workspaceID, nil
}

func findAutomationTemplate(ctx context.Context, tx pgx.Tx, zoneID string, key string) (tenancydomain.AutomationTemplate, error) {
	row := tx.QueryRow(ctx, `
SELECT
    template.id::text,
    template.key::text,
    template.name,
    template.description,
    template.zone_kind,
    template.template_type,
    template.runtime_kind,
    template.config_schema,
    template.default_config,
    template.required_scopes,
    template.status,
    template.created_at,
    template.updated_at
FROM automation_templates template
JOIN zones zone
  ON zone.id = $1::uuid
 AND zone.status = 'active'
 AND zone.deleted_at IS NULL
WHERE template.key = $2
  AND template.status = 'active'
  AND template.deleted_at IS NULL
  AND template.zone_kind IN ('any', zone.kind)
LIMIT 1
`, zoneID, key)
	template, err := scanAutomationTemplate(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return tenancydomain.AutomationTemplate{}, tenancydomain.ErrAutomationTemplateNotFound
	}
	return template, err
}

func scanAutomationTemplate(row rowScanner) (tenancydomain.AutomationTemplate, error) {
	var template tenancydomain.AutomationTemplate
	var description sql.NullString
	var configSchema []byte
	var defaultConfig []byte
	var requiredScopes []byte
	if err := row.Scan(
		&template.ID,
		&template.Key,
		&template.Name,
		&description,
		&template.ZoneKind,
		&template.TemplateType,
		&template.RuntimeKind,
		&configSchema,
		&defaultConfig,
		&requiredScopes,
		&template.Status,
		&template.CreatedAt,
		&template.UpdatedAt,
	); err != nil {
		return tenancydomain.AutomationTemplate{}, err
	}
	template.Description = nullStringPtr(description)
	template.ConfigSchema = decodeMap(configSchema)
	template.DefaultConfig = decodeMap(defaultConfig)
	_ = json.Unmarshal(requiredScopes, &template.RequiredScopes)
	if template.RequiredScopes == nil {
		template.RequiredScopes = []string{}
	}
	return template, nil
}

func scanAutomationInstallation(row rowScanner) (tenancydomain.AutomationInstallation, error) {
	var installation tenancydomain.AutomationInstallation
	var workspaceID sql.NullString
	var templateID sql.NullString
	var templateKey sql.NullString
	var config []byte
	var secretRef sql.NullString
	var runtimeWebhookID sql.NullString
	var installedBy sql.NullString
	if err := row.Scan(
		&installation.ID,
		&installation.ZoneID,
		&workspaceID,
		&templateID,
		&templateKey,
		&installation.Name,
		&installation.Status,
		&config,
		&secretRef,
		&runtimeWebhookID,
		&installedBy,
		&installation.CreatedAt,
		&installation.UpdatedAt,
	); err != nil {
		return tenancydomain.AutomationInstallation{}, err
	}
	installation.WorkspaceID = nullStringPtr(workspaceID)
	installation.TemplateID = nullStringPtr(templateID)
	installation.TemplateKey = nullStringPtr(templateKey)
	installation.Config = decodeMap(config)
	installation.SecretRef = nullStringPtr(secretRef)
	installation.RuntimeWebhookID = nullStringPtr(runtimeWebhookID)
	installation.InstalledBy = nullStringPtr(installedBy)
	return installation, nil
}

func mergeConfig(defaults map[string]any, overrides map[string]any) map[string]any {
	result := make(map[string]any, len(defaults)+len(overrides))
	for key, value := range defaults {
		result[key] = value
	}
	for key, value := range overrides {
		result[key] = value
	}
	return result
}

func automationRuntimeValues(config map[string]any) (string, []string, error) {
	targetURL, _ := config["endpoint_url"].(string)
	targetURL = strings.TrimSpace(targetURL)
	parsed, err := url.Parse(targetURL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return "", nil, fmt.Errorf(
			"%w: endpoint_url must be a valid HTTPS URL without user info",
			tenancydomain.ErrAutomationConfigInvalid,
		)
	}
	values := make([]string, 0)
	switch typed := config["event_types"].(type) {
	case []string:
		values = append(values, typed...)
	case []any:
		for _, item := range typed {
			if value, ok := item.(string); ok {
				values = append(values, value)
			}
		}
	case string:
		values = append(values, typed)
	}
	seen := make(map[string]struct{}, len(values))
	eventTypes := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || len(value) > 120 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		eventTypes = append(eventTypes, value)
	}
	if len(eventTypes) == 0 {
		eventTypes = []string{"MessageCreated"}
	}
	sort.Strings(eventTypes)
	return targetURL, eventTypes, nil
}

func ensureAutomationQuota(ctx context.Context, tx pgx.Tx, zoneID string) error {
	var enforcementMode string
	var maximum int
	var current int
	err := tx.QueryRow(ctx, `
SELECT quota.enforcement_mode,
       quota.max_automation_installations,
       (
           SELECT count(*)
           FROM automation_installations installation
           WHERE installation.zone_id = quota.zone_id
             AND installation.deleted_at IS NULL
       )
FROM zone_quotas quota
WHERE quota.zone_id = $1::uuid
FOR UPDATE
`, zoneID).Scan(&enforcementMode, &maximum, &current)
	if errors.Is(err, pgx.ErrNoRows) {
		return tenancydomain.ErrZoneNotFound
	}
	if err != nil {
		return err
	}
	if enforcementMode == "hard" && current >= maximum {
		return tenancydomain.ErrZoneQuotaExceeded
	}
	return nil
}

func validateAutomationChannel(ctx context.Context, tx pgx.Tx, workspaceID string, config map[string]any) error {
	channelSlug, _ := config["channel_slug"].(string)
	channelSlug = strings.TrimSpace(channelSlug)
	if channelSlug == "" {
		return nil
	}
	var exists bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM channels
    WHERE workspace_id = $1::uuid
      AND slug = $2
      AND status = 'active'
      AND deleted_at IS NULL
)
`, workspaceID, channelSlug).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf(
			"%w: channel_slug does not exist in the selected workspace",
			tenancydomain.ErrAutomationConfigInvalid,
		)
	}
	return nil
}

func decodeMap(raw []byte) map[string]any {
	result := map[string]any{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &result)
	}
	return result
}

func zoneStorageBucket(zoneID string) string {
	compact := strings.ReplaceAll(zoneID, "-", "")
	if len(compact) > 16 {
		compact = compact[:16]
	}
	return "webtui-zone-" + compact
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}
