package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	tenancyapp "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/application"
	tenancydomain "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/domain"
	"github.com/jackc/pgx/v5"
)

func (r *Repository) CreateAdditionalDomainClaim(
	ctx context.Context,
	params tenancyapp.CreateAdditionalDomainClaimParams,
) (tenancydomain.DomainClaim, error) {
	var zoneActive bool
	if err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM zones
    WHERE id = $1::uuid
      AND status = 'active'
      AND deleted_at IS NULL
)
`, params.ZoneID).Scan(&zoneActive); err != nil {
		return tenancydomain.DomainClaim{}, err
	}
	if !zoneActive {
		return tenancydomain.DomainClaim{}, tenancydomain.ErrZoneNotFound
	}

	var domainID string
	err := r.pool.QueryRow(ctx, `
INSERT INTO zone_domains (
    zone_id, domain, kind, status, verification_method,
    verification_expires_at, tls_status, created_by, metadata
)
VALUES (
    $1::uuid, $2, $3, 'pending', 'dns_txt',
    $4, 'pending', $5::uuid, '{"claim_source":"zone_admin"}'::jsonb
)
RETURNING id::text
`, params.ZoneID, params.Domain, params.Kind, params.ExpiresAt, params.ActorUserID).Scan(&domainID)
	if err != nil {
		if isUniqueViolation(err) {
			return tenancydomain.DomainClaim{}, tenancydomain.ErrDomainAlreadyClaimed
		}
		return tenancydomain.DomainClaim{}, err
	}
	return r.FindDomainClaim(ctx, domainID, params.ActorUserID)
}

func (r *Repository) ActivateVerifiedDomain(
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
	if claim.Zone.Status != "active" || claim.Workspace == nil {
		return tenancydomain.ResolvedZone{}, tenancydomain.ErrZoneNotFound
	}
	if claim.Domain.Status == "active" {
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
	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (
    zone_id, workspace_id, actor_user_id, action, entity_type, entity_id, metadata
)
VALUES (
    $1::uuid, $2::uuid, $3::uuid, 'zone.domain_verified',
    'zone_domain', $4::uuid,
    jsonb_build_object('domain', $5, 'kind', $6, 'verification_method', 'dns_txt')
)
`, claim.Zone.ID, claim.Workspace.ID, params.ActorUserID, claim.Domain.ID,
		claim.Domain.Domain, claim.Domain.Kind); err != nil {
		return tenancydomain.ResolvedZone{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return tenancydomain.ResolvedZone{}, err
	}
	return r.ResolveByDomain(ctx, claim.Domain.Domain)
}

func (r *Repository) GetZoneAdminOverview(
	ctx context.Context,
	zoneID string,
) (tenancydomain.ZoneAdminOverview, error) {
	var overview tenancydomain.ZoneAdminOverview
	var primaryWorkspaceID sql.NullString
	var metadataJSON []byte
	err := r.pool.QueryRow(ctx, `
SELECT id::text, slug::text, name, kind, status, registration_mode,
       primary_workspace_id::text, metadata, created_at, updated_at
FROM zones
WHERE id = $1::uuid
  AND deleted_at IS NULL
`, zoneID).Scan(
		&overview.Zone.ID,
		&overview.Zone.Slug,
		&overview.Zone.Name,
		&overview.Zone.Kind,
		&overview.Zone.Status,
		&overview.Zone.RegistrationMode,
		&primaryWorkspaceID,
		&metadataJSON,
		&overview.Zone.CreatedAt,
		&overview.Zone.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return tenancydomain.ZoneAdminOverview{}, tenancydomain.ErrZoneNotFound
	}
	if err != nil {
		return tenancydomain.ZoneAdminOverview{}, err
	}
	overview.Zone.PrimaryWorkspaceID = nullStringPtr(primaryWorkspaceID)
	overview.Zone.Metadata = decodeMap(metadataJSON)

	domainRows, err := r.pool.Query(ctx, `
SELECT id::text, zone_id::text, domain::text, kind, status,
       verification_method, verification_token, verified_at,
       verification_expires_at, verification_attempts,
       last_verification_error, tls_status, last_checked_at,
       created_at, updated_at
FROM zone_domains
WHERE zone_id = $1::uuid
  AND deleted_at IS NULL
ORDER BY CASE kind WHEN 'primary' THEN 0 ELSE 1 END, created_at
`, zoneID)
	if err != nil {
		return tenancydomain.ZoneAdminOverview{}, err
	}
	defer domainRows.Close()
	overview.Domains = make([]tenancydomain.Domain, 0)
	for domainRows.Next() {
		domain, scanErr := scanZoneDomain(domainRows)
		if scanErr != nil {
			return tenancydomain.ZoneAdminOverview{}, scanErr
		}
		overview.Domains = append(overview.Domains, domain)
	}
	if err := domainRows.Err(); err != nil {
		return tenancydomain.ZoneAdminOverview{}, err
	}

	deploymentRows, err := r.pool.Query(ctx, `
SELECT id::text, zone_id::text, mode, web_base_url, api_base_url, ws_base_url,
       admin_base_url, database_mode, storage_bucket, redis_prefix, status, metadata
FROM zone_deployments
WHERE zone_id = $1::uuid
  AND deleted_at IS NULL
ORDER BY created_at DESC
`, zoneID)
	if err != nil {
		return tenancydomain.ZoneAdminOverview{}, err
	}
	defer deploymentRows.Close()
	overview.Deployments = make([]tenancydomain.Deployment, 0)
	for deploymentRows.Next() {
		deployment, scanErr := scanZoneDeployment(deploymentRows)
		if scanErr != nil {
			return tenancydomain.ZoneAdminOverview{}, scanErr
		}
		overview.Deployments = append(overview.Deployments, deployment)
	}
	return overview, deploymentRows.Err()
}

func (r *Repository) UpdateZoneSettings(
	ctx context.Context,
	params tenancyapp.UpdateZoneSettingsParams,
) (tenancydomain.ZoneAdminOverview, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return tenancydomain.ZoneAdminOverview{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	name := ""
	if params.Name != nil {
		name = *params.Name
	}
	registrationMode := ""
	if params.RegistrationMode != nil {
		registrationMode = *params.RegistrationMode
	}
	command, err := tx.Exec(ctx, `
UPDATE zones
SET name = CASE WHEN $2::boolean THEN $3 ELSE name END,
    registration_mode = CASE WHEN $4::boolean THEN $5 ELSE registration_mode END
WHERE id = $1::uuid
  AND status <> 'archived'
  AND deleted_at IS NULL
`, params.ZoneID, params.Name != nil, name,
		params.RegistrationMode != nil, registrationMode)
	if err != nil {
		return tenancydomain.ZoneAdminOverview{}, err
	}
	if command.RowsAffected() == 0 {
		return tenancydomain.ZoneAdminOverview{}, tenancydomain.ErrZoneNotFound
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (zone_id, actor_user_id, action, entity_type, entity_id, metadata)
VALUES (
    $1::uuid, $2::uuid, 'zone.settings_updated', 'zone', $1::uuid,
    jsonb_build_object('name_changed', $3::boolean, 'registration_mode_changed', $4::boolean)
)
`, params.ZoneID, params.ActorUserID, params.Name != nil, params.RegistrationMode != nil); err != nil {
		return tenancydomain.ZoneAdminOverview{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return tenancydomain.ZoneAdminOverview{}, err
	}
	return r.GetZoneAdminOverview(ctx, params.ZoneID)
}

func (r *Repository) SetZoneLifecycle(
	ctx context.Context,
	params tenancyapp.SetZoneLifecycleParams,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var kind string
	var status string
	var workspaceID sql.NullString
	if err := tx.QueryRow(ctx, `
SELECT kind, status, primary_workspace_id::text
FROM zones
WHERE id = $1::uuid
  AND deleted_at IS NULL
FOR UPDATE
`, params.ZoneID).Scan(&kind, &status, &workspaceID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tenancydomain.ErrZoneNotFound
		}
		return err
	}
	if kind == "vpsttt_internal" {
		return tenancydomain.ErrZoneAccessDenied
	}

	nextStatus := status
	workspaceStatus := "active"
	deploymentStatus := "ready"
	switch params.Action {
	case "suspend":
		if status == "archived" {
			return tenancydomain.ErrZoneNotFound
		}
		nextStatus = "suspended"
		workspaceStatus = "suspended"
		deploymentStatus = "suspended"
	case "resume":
		if status != "suspended" {
			return tenancydomain.ErrZoneNotFound
		}
		nextStatus = "active"
	case "archive":
		nextStatus = "archived"
		workspaceStatus = "archived"
		deploymentStatus = "suspended"
	}

	if _, err := tx.Exec(ctx, `
UPDATE zones
SET status = $2,
    suspended_at = CASE WHEN $2 = 'suspended' THEN now() ELSE NULL END,
    archived_at = CASE WHEN $2 = 'archived' THEN now() ELSE archived_at END,
    lifecycle_reason = NULLIF($3, '')
WHERE id = $1::uuid
`, params.ZoneID, nextStatus, params.Reason); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE workspaces
SET status = $2
WHERE zone_id = $1::uuid
  AND deleted_at IS NULL
`, params.ZoneID, workspaceStatus); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE zone_deployments
SET status = $2
WHERE zone_id = $1::uuid
  AND deleted_at IS NULL
`, params.ZoneID, deploymentStatus); err != nil {
		return err
	}
	if nextStatus == "archived" {
		if _, err := tx.Exec(ctx, `
UPDATE zone_domains
SET status = 'suspended',
    tls_status = 'disabled'
WHERE zone_id = $1::uuid
  AND deleted_at IS NULL
`, params.ZoneID); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `
UPDATE automation_installations
SET status = 'disabled'
WHERE zone_id = $1::uuid
  AND deleted_at IS NULL
`, params.ZoneID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
UPDATE outgoing_webhooks webhook
SET status = CASE
    WHEN $2 = 'active' AND installation.status = 'enabled' THEN 'active'
    ELSE 'disabled'
END
FROM automation_installations installation
WHERE installation.runtime_webhook_id = webhook.id
  AND installation.zone_id = $1::uuid
  AND installation.deleted_at IS NULL
`, params.ZoneID, nextStatus); err != nil {
		return err
	}
	if nextStatus != status {
		if _, err := tx.Exec(ctx, `
UPDATE user_sessions
SET revoked_at = COALESCE(revoked_at, now())
WHERE zone_id = $1::uuid
  AND revoked_at IS NULL
`, params.ZoneID); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (
    zone_id, workspace_id, actor_user_id, action, entity_type, entity_id, metadata
)
VALUES (
    $1::uuid, $2::uuid, $3::uuid, $4, 'zone', $1::uuid,
    jsonb_build_object('previous_status', $5, 'next_status', $6, 'reason', NULLIF($7, ''))
)
`, params.ZoneID, nullableUUID(workspaceID), params.ActorUserID,
		"zone."+params.Action, status, nextStatus, params.Reason); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) SetPrimaryDomain(
	ctx context.Context,
	zoneID string,
	domainID string,
	actorUserID string,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	var domain string
	var currentKind string
	if err := tx.QueryRow(ctx, `
SELECT domain::text, kind
FROM zone_domains
WHERE id = $1::uuid
  AND zone_id = $2::uuid
  AND status = 'active'
  AND deleted_at IS NULL
FOR UPDATE
`, domainID, zoneID).Scan(&domain, &currentKind); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tenancydomain.ErrDomainClaimNotFound
		}
		return err
	}
	if currentKind == "primary" {
		return tx.Commit(ctx)
	}
	if _, err := tx.Exec(ctx, `
UPDATE zone_domains
SET kind = 'alias'
WHERE zone_id = $1::uuid
  AND kind = 'primary'
  AND deleted_at IS NULL
`, zoneID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE zone_domains
SET kind = 'primary'
WHERE id = $1::uuid
`, domainID); err != nil {
		return err
	}
	baseURL := "https://" + domain
	if _, err := tx.Exec(ctx, `
UPDATE zone_deployments
SET web_base_url = $2,
    api_base_url = $2,
    ws_base_url = $3,
    admin_base_url = $4
WHERE zone_id = $1::uuid
  AND mode = 'shared'
  AND deleted_at IS NULL
`, zoneID, baseURL, "wss://"+domain+"/ws", baseURL+"/admin"); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (zone_id, actor_user_id, action, entity_type, entity_id, metadata)
VALUES (
    $1::uuid, $2::uuid, 'zone.primary_domain_changed', 'zone_domain', $3::uuid,
    jsonb_build_object('domain', $4)
)
`, zoneID, actorUserID, domainID, domain); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) DeleteZoneDomain(
	ctx context.Context,
	zoneID string,
	domainID string,
	actorUserID string,
) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	var kind string
	var domain string
	if err := tx.QueryRow(ctx, `
SELECT kind, domain::text
FROM zone_domains
WHERE id = $1::uuid
  AND zone_id = $2::uuid
  AND deleted_at IS NULL
FOR UPDATE
`, domainID, zoneID).Scan(&kind, &domain); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return tenancydomain.ErrDomainClaimNotFound
		}
		return err
	}
	if kind == "primary" {
		return tenancydomain.ErrDomainIsPrimary
	}
	if _, err := tx.Exec(ctx, `
UPDATE zone_domains
SET status = 'suspended',
    tls_status = 'disabled',
    deleted_at = now()
WHERE id = $1::uuid
`, domainID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (zone_id, actor_user_id, action, entity_type, entity_id, metadata)
VALUES (
    $1::uuid, $2::uuid, 'zone.domain_deleted', 'zone_domain', $3::uuid,
    jsonb_build_object('domain', $4, 'kind', $5)
)
`, zoneID, actorUserID, domainID, domain, kind); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *Repository) ListDeploymentRequests(
	ctx context.Context,
	zoneID string,
) ([]tenancydomain.DeploymentRequest, error) {
	rows, err := r.pool.Query(ctx, deploymentRequestSelect+`
WHERE zone_id = $1::uuid
ORDER BY created_at DESC
`, zoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	requests := make([]tenancydomain.DeploymentRequest, 0)
	for rows.Next() {
		request, scanErr := scanDeploymentRequest(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		requests = append(requests, request)
	}
	return requests, rows.Err()
}

func (r *Repository) CreateDeploymentRequest(
	ctx context.Context,
	params tenancyapp.CreateDeploymentRequestParams,
) (tenancydomain.DeploymentRequest, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return tenancydomain.DeploymentRequest{}, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()
	metadata := `{"driver_status":"awaiting_external_provisioner"}`
	status := "pending"
	if params.RequestedMode == "shared" && params.RequestedDatabaseMode == "shared_schema" {
		metadata = `{"driver_status":"built_in_shared_runtime"}`
		status = "ready"
	}
	row := tx.QueryRow(ctx, `
INSERT INTO zone_deployment_requests (
    zone_id, requested_mode, requested_database_mode, status,
    idempotency_key, requested_by, metadata, completed_at
)
VALUES (
    $1::uuid, $2, $3, $4, $5, $6::uuid, $7::jsonb,
    CASE WHEN $4 = 'ready' THEN now() ELSE NULL END
)
ON CONFLICT (zone_id, idempotency_key) DO UPDATE
SET idempotency_key = EXCLUDED.idempotency_key
RETURNING
    id::text, zone_id::text, requested_mode, requested_database_mode,
    status, idempotency_key, requested_by::text, failure_reason,
    metadata, created_at, updated_at, completed_at
`, params.ZoneID, params.RequestedMode, params.RequestedDatabaseMode, status,
		params.IdempotencyKey, params.ActorUserID, metadata)
	request, err := scanDeploymentRequest(row)
	if err != nil {
		return tenancydomain.DeploymentRequest{}, err
	}
	if request.RequestedMode != params.RequestedMode ||
		request.RequestedDatabaseMode != params.RequestedDatabaseMode {
		return tenancydomain.DeploymentRequest{}, tenancydomain.ErrDeploymentRequestConflict
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (zone_id, actor_user_id, action, entity_type, entity_id, metadata)
VALUES (
    $1::uuid, $2::uuid, 'zone.deployment_requested',
    'zone_deployment_request', $3::uuid,
    jsonb_build_object(
        'requested_mode', $4,
        'requested_database_mode', $5,
        'status', $6,
        'idempotency_key', $7
    )
)
`, params.ZoneID, params.ActorUserID, request.ID, request.RequestedMode,
		request.RequestedDatabaseMode, request.Status, request.IdempotencyKey); err != nil {
		return tenancydomain.DeploymentRequest{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return tenancydomain.DeploymentRequest{}, err
	}
	return request, nil
}

const deploymentRequestSelect = `
SELECT
    id::text, zone_id::text, requested_mode, requested_database_mode,
    status, idempotency_key, requested_by::text, failure_reason,
    metadata, created_at, updated_at, completed_at
FROM zone_deployment_requests
`

func scanDeploymentRequest(row rowScanner) (tenancydomain.DeploymentRequest, error) {
	var request tenancydomain.DeploymentRequest
	var requestedBy sql.NullString
	var failureReason sql.NullString
	var metadataJSON []byte
	var completedAt sql.NullTime
	err := row.Scan(
		&request.ID,
		&request.ZoneID,
		&request.RequestedMode,
		&request.RequestedDatabaseMode,
		&request.Status,
		&request.IdempotencyKey,
		&requestedBy,
		&failureReason,
		&metadataJSON,
		&request.CreatedAt,
		&request.UpdatedAt,
		&completedAt,
	)
	if err != nil {
		return tenancydomain.DeploymentRequest{}, err
	}
	request.RequestedBy = nullStringPtr(requestedBy)
	request.FailureReason = nullStringPtr(failureReason)
	request.Metadata = decodeMap(metadataJSON)
	request.CompletedAt = nullTimePtr(completedAt)
	return request, nil
}

func scanZoneDomain(row rowScanner) (tenancydomain.Domain, error) {
	var domain tenancydomain.Domain
	var verifiedAt sql.NullTime
	var verificationExpiresAt sql.NullTime
	var lastVerificationError sql.NullString
	var lastCheckedAt sql.NullTime
	err := row.Scan(
		&domain.ID,
		&domain.ZoneID,
		&domain.Domain,
		&domain.Kind,
		&domain.Status,
		&domain.VerificationMethod,
		&domain.VerificationToken,
		&verifiedAt,
		&verificationExpiresAt,
		&domain.VerificationAttempts,
		&lastVerificationError,
		&domain.TLSStatus,
		&lastCheckedAt,
		&domain.CreatedAt,
		&domain.UpdatedAt,
	)
	if err != nil {
		return tenancydomain.Domain{}, err
	}
	domain.VerifiedAt = nullTimePtr(verifiedAt)
	domain.VerificationExpiresAt = nullTimePtr(verificationExpiresAt)
	domain.LastVerificationError = nullStringPtr(lastVerificationError)
	domain.LastCheckedAt = nullTimePtr(lastCheckedAt)
	return domain, nil
}

func scanZoneDeployment(row rowScanner) (tenancydomain.Deployment, error) {
	var deployment tenancydomain.Deployment
	var adminBaseURL sql.NullString
	var storageBucket sql.NullString
	var redisPrefix sql.NullString
	var metadataJSON []byte
	err := row.Scan(
		&deployment.ID,
		&deployment.ZoneID,
		&deployment.Mode,
		&deployment.WebBaseURL,
		&deployment.APIBaseURL,
		&deployment.WSBaseURL,
		&adminBaseURL,
		&deployment.DatabaseMode,
		&storageBucket,
		&redisPrefix,
		&deployment.Status,
		&metadataJSON,
	)
	if err != nil {
		return tenancydomain.Deployment{}, err
	}
	deployment.AdminBaseURL = nullStringPtr(adminBaseURL)
	deployment.StorageBucket = nullStringPtr(storageBucket)
	deployment.RedisPrefix = nullStringPtr(redisPrefix)
	deployment.Metadata = decodeMap(metadataJSON)
	return deployment, nil
}

func nullableUUID(value sql.NullString) any {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil
	}
	return value.String
}
