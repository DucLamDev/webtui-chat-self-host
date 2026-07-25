package postgres

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var selfHostedSlugSanitizer = regexp.MustCompile(`[^a-z0-9]+`)

type SelfHostedInstanceParams struct {
	Domain string
	Name   string
}

// EnsureSelfHostedInstance converts the bootstrap zone created by migrations
// into the single customer-owned instance configured by the operator.
func EnsureSelfHostedInstance(
	ctx context.Context,
	pool *pgxpool.Pool,
	params SelfHostedInstanceParams,
) (string, error) {
	domain := strings.ToLower(strings.TrimSpace(params.Domain))
	name := strings.TrimSpace(params.Name)
	if domain == "" || name == "" {
		return "", errors.New("self-hosted instance domain and name are required")
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var zoneCount int
	if err := tx.QueryRow(ctx, `
SELECT count(*)
FROM zones
WHERE deleted_at IS NULL
`).Scan(&zoneCount); err != nil {
		return "", fmt.Errorf("count zones before self-hosted bootstrap: %w", err)
	}
	if zoneCount != 1 {
		return "", fmt.Errorf(
			"self-hosted mode requires exactly one zone, found %d; use a fresh database or migrate each customer separately",
			zoneCount,
		)
	}

	var zoneID string
	var workspaceID string
	var hasOwner bool
	var alreadySelfHosted bool
	err = tx.QueryRow(ctx, `
SELECT
    zone.id::text,
    workspace.id::text,
    workspace.owner_id IS NOT NULL,
    COALESCE(zone.metadata->>'deployment_model' = 'self_hosted', false)
FROM zones zone
JOIN workspaces workspace
  ON workspace.id = zone.primary_workspace_id
 AND workspace.zone_id = zone.id
 AND workspace.deleted_at IS NULL
WHERE zone.deleted_at IS NULL
  AND (
      zone.metadata->>'deployment_model' = 'self_hosted'
      OR zone.kind = 'vpsttt_internal'
  )
FOR UPDATE OF zone, workspace
`).Scan(&zoneID, &workspaceID, &hasOwner, &alreadySelfHosted)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errors.New("self-hosted bootstrap zone is unavailable; run all database migrations first")
	}
	if err != nil {
		return "", fmt.Errorf("lock self-hosted bootstrap zone: %w", err)
	}

	registrationMode := "open"
	if hasOwner {
		registrationMode = "invite_only"
	}
	slug := selfHostedSlug(domain)
	baseURL := "https://" + domain

	if _, err := tx.Exec(ctx, `
UPDATE zones
SET slug = $2,
    name = $3,
    kind = 'customer_dedicated',
    status = 'active',
    registration_mode = $4,
    metadata = (COALESCE(metadata, '{}'::jsonb) - 'template_key') || jsonb_build_object(
        'deployment_model', 'self_hosted',
        'managed_by', 'customer',
        'capabilities', jsonb_build_object(
            'chat', true,
            'files', true,
            'calls', true,
            'bots', true,
            'automation', true,
            'webhooks', true,
            'federation', false,
            'sso', false
        )
    )
WHERE id = $1::uuid
`, zoneID, slug, name, registrationMode); err != nil {
		return "", fmt.Errorf("configure self-hosted zone: %w", err)
	}

	if _, err := tx.Exec(ctx, `
UPDATE workspaces
SET name = $2,
    description = 'Primary workspace of the self-hosted instance ' || $3,
    plan = 'self_hosted',
    status = 'active'
WHERE id = $1::uuid
`, workspaceID, name, domain); err != nil {
		return "", fmt.Errorf("configure self-hosted workspace: %w", err)
	}

	if !alreadySelfHosted && !hasOwner {
		if _, err := tx.Exec(ctx, `
DELETE FROM bots
WHERE workspace_id = $1::uuid
  AND settings @> '{"system_default":true}'::jsonb
`, workspaceID); err != nil {
			return "", fmt.Errorf("remove VPSTTT bootstrap bots: %w", err)
		}
		if _, err := tx.Exec(ctx, `
DELETE FROM channels
WHERE workspace_id = $1::uuid
  AND settings @> '{"system_default":true}'::jsonb
`, workspaceID); err != nil {
			return "", fmt.Errorf("remove VPSTTT bootstrap channels: %w", err)
		}
		if _, err := tx.Exec(ctx, `
INSERT INTO channels (
    workspace_id, slug, name, description, type, created_by, settings
)
SELECT
    $1::uuid,
    definition.slug,
    definition.name,
    definition.description,
    'public',
    NULL,
    '{"system_default":true,"template_key":"customer_standard"}'::jsonb
FROM (
    VALUES
        ('general', 'General', 'Trao doi chung cua workspace'),
        ('announcements', 'Announcements', 'Thong bao cua workspace'),
        ('random', 'Random', 'Trao doi ngoai cong viec'),
        ('automation', 'Automation', 'Su kien tu bot, webhook va automation')
) AS definition(slug, name, description)
ON CONFLICT (workspace_id, slug)
    WHERE slug IS NOT NULL AND deleted_at IS NULL
DO NOTHING
`, workspaceID); err != nil {
			return "", fmt.Errorf("create customer bootstrap channels: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `
UPDATE zone_domains
SET status = 'suspended',
    deleted_at = COALESCE(deleted_at, now())
WHERE zone_id = $1::uuid
  AND kind <> 'primary'
  AND deleted_at IS NULL
`, zoneID); err != nil {
		return "", fmt.Errorf("retire self-hosted domain aliases: %w", err)
	}

	var primaryDomainID string
	err = tx.QueryRow(ctx, `
SELECT id::text
FROM zone_domains
WHERE zone_id = $1::uuid
  AND kind = 'primary'
  AND deleted_at IS NULL
ORDER BY created_at
LIMIT 1
FOR UPDATE
`, zoneID).Scan(&primaryDomainID)
	if errors.Is(err, pgx.ErrNoRows) {
		err = tx.QueryRow(ctx, `
INSERT INTO zone_domains (
    zone_id, domain, kind, status, verification_method,
    verification_token, verified_at, tls_status, metadata
)
VALUES (
    $1::uuid, $2, 'primary', 'active', 'manual',
    'self-hosted-configured', now(), 'ready',
    '{"managed_by":"self_hosted_config"}'::jsonb
)
RETURNING id::text
`, zoneID, domain).Scan(&primaryDomainID)
	} else if err == nil {
		_, err = tx.Exec(ctx, `
UPDATE zone_domains
SET domain = $2,
    status = 'active',
    verification_method = 'manual',
    verification_token = 'self-hosted-configured',
    verified_at = now(),
    tls_status = 'ready',
    verification_expires_at = NULL,
    last_verification_error = NULL,
    metadata = COALESCE(metadata, '{}'::jsonb) || '{"managed_by":"self_hosted_config"}'::jsonb
WHERE id = $1::uuid
`, primaryDomainID, domain)
	}
	if err != nil {
		return "", fmt.Errorf("configure self-hosted primary domain: %w", err)
	}

	if _, err := tx.Exec(ctx, `
UPDATE zone_domains
SET status = 'suspended',
    deleted_at = COALESCE(deleted_at, now())
WHERE zone_id = $1::uuid
  AND id <> $2::uuid
  AND deleted_at IS NULL
`, zoneID, primaryDomainID); err != nil {
		return "", fmt.Errorf("retire non-configured self-hosted domains: %w", err)
	}

	if _, err := tx.Exec(ctx, `
WITH selected AS (
    SELECT id
    FROM zone_deployments
    WHERE zone_id = $1::uuid
      AND deleted_at IS NULL
    ORDER BY created_at
    LIMIT 1
)
UPDATE zone_deployments deployment
SET mode = 'dedicated_compose',
    web_base_url = $2,
    api_base_url = $2,
    ws_base_url = $3,
    admin_base_url = $4,
    database_mode = 'dedicated_database',
    storage_bucket = 'webtui-chat',
    redis_prefix = 'instance',
    status = 'ready',
    metadata = COALESCE(metadata, '{}'::jsonb) || '{"deployment_model":"self_hosted"}'::jsonb
FROM selected
WHERE deployment.id = selected.id
`, zoneID, baseURL, "wss://"+domain+"/ws", baseURL+"/admin"); err != nil {
		return "", fmt.Errorf("update self-hosted deployment: %w", err)
	}

	var deploymentExists bool
	if err := tx.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM zone_deployments
    WHERE zone_id = $1::uuid
      AND deleted_at IS NULL
)
`, zoneID).Scan(&deploymentExists); err != nil {
		return "", fmt.Errorf("check self-hosted deployment: %w", err)
	}
	if !deploymentExists {
		if _, err := tx.Exec(ctx, `
INSERT INTO zone_deployments (
    zone_id, mode, web_base_url, api_base_url, ws_base_url,
    admin_base_url, database_mode, storage_bucket, redis_prefix, status, metadata
)
VALUES (
    $1::uuid, 'dedicated_compose', $2, $2, $3,
    $4, 'dedicated_database', 'webtui-chat', 'instance', 'ready',
    '{"deployment_model":"self_hosted"}'::jsonb
)
`, zoneID, baseURL, "wss://"+domain+"/ws", baseURL+"/admin"); err != nil {
			return "", fmt.Errorf("create self-hosted deployment: %w", err)
		}
	}

	if _, err := tx.Exec(ctx, `
UPDATE zone_deployments
SET deleted_at = now()
WHERE zone_id = $1::uuid
  AND deleted_at IS NULL
  AND id NOT IN (
      SELECT id
      FROM zone_deployments
      WHERE zone_id = $1::uuid
        AND deleted_at IS NULL
      ORDER BY created_at
      LIMIT 1
  )
`, zoneID); err != nil {
		return "", fmt.Errorf("retire duplicate self-hosted deployments: %w", err)
	}

	if _, err := tx.Exec(ctx, `
UPDATE zone_quotas
SET max_workspaces = 100000,
    max_members = 1000000,
    max_storage_bytes = 9223372036854775807,
    max_automation_installations = 100000,
    max_webhooks = 100000,
    enforcement_mode = 'monitor'
WHERE zone_id = $1::uuid
`, zoneID); err != nil {
		return "", fmt.Errorf("configure self-hosted quotas: %w", err)
	}

	if _, err := tx.Exec(ctx, `
UPDATE channels
SET settings = (COALESCE(settings, '{}'::jsonb) - 'template_key') || '{"template_key":"customer_standard"}'::jsonb
WHERE workspace_id = $1::uuid
  AND deleted_at IS NULL
`, workspaceID); err != nil {
		return "", fmt.Errorf("configure self-hosted channels: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return workspaceID, nil
}

func selfHostedSlug(domain string) string {
	slug := strings.Trim(selfHostedSlugSanitizer.ReplaceAllString(
		strings.ToLower(strings.TrimSpace(domain)),
		"-",
	), "-")
	if slug == "" {
		return "instance"
	}
	if len(slug) > 63 {
		slug = strings.Trim(slug[:63], "-")
	}
	return slug
}
