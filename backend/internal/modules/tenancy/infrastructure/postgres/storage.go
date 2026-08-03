package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	tenancyapp "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/application"
	tenancydomain "github.com/duclamdev/application-chat/backend/internal/modules/tenancy/domain"
)

func (r *Repository) GetZoneStorageConfig(
	ctx context.Context,
	zoneID string,
) (tenancydomain.ZoneStorageConfig, error) {
	var config tenancydomain.ZoneStorageConfig
	err := r.pool.QueryRow(ctx, `
SELECT zone_id::text, provider, COALESCE(endpoint, ''), COALESCE(region, ''),
       COALESCE(bucket, ''), COALESCE(access_key_id, ''),
       COALESCE(secret_access_key_encrypted, ''), created_at, updated_at
FROM zone_storage_configs
WHERE zone_id = $1::uuid
`, zoneID).Scan(
		&config.ZoneID,
		&config.Provider,
		&config.Endpoint,
		&config.Region,
		&config.Bucket,
		&config.AccessKeyID,
		&config.SecretAccessKeyEncrypted,
		&config.CreatedAt,
		&config.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return tenancydomain.ZoneStorageConfig{}, tenancydomain.ErrZoneStorageConfigNotFound
	}
	return config, err
}

func (r *Repository) UpsertZoneStorageConfig(
	ctx context.Context,
	params tenancyapp.UpsertZoneStorageConfigParams,
) (tenancydomain.ZoneStorageConfig, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return tenancydomain.ZoneStorageConfig{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var config tenancydomain.ZoneStorageConfig
	err = tx.QueryRow(ctx, `
INSERT INTO zone_storage_configs (
    zone_id, provider, endpoint, region, bucket, access_key_id,
    secret_access_key_encrypted, created_by, updated_by
)
VALUES (
    $1::uuid, $2,
    NULLIF($3, ''), NULLIF($4, ''), NULLIF($5, ''), NULLIF($6, ''),
    NULLIF($7, ''), $8::uuid, $8::uuid
)
ON CONFLICT (zone_id) DO UPDATE SET
    provider = EXCLUDED.provider,
    endpoint = EXCLUDED.endpoint,
    region = EXCLUDED.region,
    bucket = EXCLUDED.bucket,
    access_key_id = EXCLUDED.access_key_id,
    secret_access_key_encrypted = EXCLUDED.secret_access_key_encrypted,
    updated_by = EXCLUDED.updated_by
RETURNING zone_id::text, provider, COALESCE(endpoint, ''), COALESCE(region, ''),
          COALESCE(bucket, ''), COALESCE(access_key_id, ''),
          COALESCE(secret_access_key_encrypted, ''), created_at, updated_at
`, params.ZoneID, params.Provider, params.Endpoint, params.Region, params.Bucket,
		params.AccessKeyID, params.SecretAccessKeyEncrypted, params.ActorUserID).Scan(
		&config.ZoneID,
		&config.Provider,
		&config.Endpoint,
		&config.Region,
		&config.Bucket,
		&config.AccessKeyID,
		&config.SecretAccessKeyEncrypted,
		&config.CreatedAt,
		&config.UpdatedAt,
	)
	if err != nil {
		return tenancydomain.ZoneStorageConfig{}, err
	}

	if _, err := tx.Exec(ctx, `
INSERT INTO audit_logs (zone_id, actor_user_id, action, entity_type, entity_id, metadata)
VALUES (
    $1::uuid, $2::uuid, 'zone.storage_updated', 'zone', $1::uuid,
    jsonb_build_object('provider', $3::text, 'bucket', NULLIF($4::text, ''))
)
`, params.ZoneID, params.ActorUserID, params.Provider, params.Bucket); err != nil {
		return tenancydomain.ZoneStorageConfig{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return tenancydomain.ZoneStorageConfig{}, err
	}
	return config, nil
}
