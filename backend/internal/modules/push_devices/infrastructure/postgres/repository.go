package postgres

import (
	"context"
	"database/sql"
	"errors"

	devicesapp "github.com/duclamdev/application-chat/backend/internal/modules/push_devices/application"
	devicesdomain "github.com/duclamdev/application-chat/backend/internal/modules/push_devices/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Upsert(ctx context.Context, params devicesapp.UpsertParams) (devicesdomain.Device, error) {
	row := r.pool.QueryRow(ctx, `
INSERT INTO push_devices (
    zone_id, user_id, workspace_id, device_id, platform, push_provider, push_token,
    notification_permission, app_version, build_number, release_channel, locale, timezone,
    status, last_seen_at, revoked_at
)
VALUES (
    $1::uuid, $2::uuid, NULLIF($3, '')::uuid, $4, $5, $6, NULLIF($7, ''),
    $8, NULLIF($9, ''), NULLIF($10, ''), NULLIF($11, ''), NULLIF($12, ''), NULLIF($13, ''),
    'active', now(), NULL
)
ON CONFLICT (zone_id, user_id, device_id)
DO UPDATE SET workspace_id = EXCLUDED.workspace_id,
              platform = EXCLUDED.platform,
              push_provider = EXCLUDED.push_provider,
              push_token = EXCLUDED.push_token,
              notification_permission = EXCLUDED.notification_permission,
              app_version = EXCLUDED.app_version,
              build_number = EXCLUDED.build_number,
              release_channel = EXCLUDED.release_channel,
              locale = EXCLUDED.locale,
              timezone = EXCLUDED.timezone,
              status = 'active',
              last_seen_at = now(),
              revoked_at = NULL
RETURNING id::text, user_id::text, workspace_id::text, device_id, platform, push_provider,
          push_token, notification_permission, app_version, build_number, release_channel,
          locale, timezone, status, last_seen_at, revoked_at, created_at, updated_at
`, params.ZoneID, params.UserID, params.WorkspaceID, params.DeviceID, params.Platform, params.PushProvider, params.PushToken,
		params.NotificationPermission, params.AppVersion, params.BuildNumber, params.ReleaseChannel, params.Locale, params.Timezone)
	return scanDevice(row)
}

func (r *Repository) ListMine(ctx context.Context, zoneID string, userID string) ([]devicesdomain.Device, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, user_id::text, workspace_id::text, device_id, platform, push_provider,
       push_token, notification_permission, app_version, build_number, release_channel,
       locale, timezone, status, last_seen_at, revoked_at, created_at, updated_at
FROM push_devices
WHERE user_id = $1::uuid
  AND zone_id = $2::uuid
ORDER BY status, updated_at DESC
`, userID, zoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	devices := make([]devicesdomain.Device, 0)
	for rows.Next() {
		device, err := scanDevice(rows)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
	}
	return devices, rows.Err()
}

func (r *Repository) Delete(ctx context.Context, zoneID string, userID string, deviceID string) error {
	command, err := r.pool.Exec(ctx, `
UPDATE push_devices
SET status = 'revoked',
    push_token = NULL,
    revoked_at = now()
WHERE zone_id = $1::uuid
  AND user_id = $2::uuid
  AND device_id = $3
  AND status <> 'revoked'
`, zoneID, userID, deviceID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return devicesdomain.ErrDeviceNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanDevice(row rowScanner) (devicesdomain.Device, error) {
	var device devicesdomain.Device
	var workspaceID sql.NullString
	var pushToken sql.NullString
	var appVersion sql.NullString
	var buildNumber sql.NullString
	var releaseChannel sql.NullString
	var locale sql.NullString
	var timezone sql.NullString
	var revokedAt sql.NullTime
	if err := row.Scan(
		&device.ID,
		&device.UserID,
		&workspaceID,
		&device.DeviceID,
		&device.Platform,
		&device.PushProvider,
		&pushToken,
		&device.NotificationPermission,
		&appVersion,
		&buildNumber,
		&releaseChannel,
		&locale,
		&timezone,
		&device.Status,
		&device.LastSeenAt,
		&revokedAt,
		&device.CreatedAt,
		&device.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return devicesdomain.Device{}, devicesdomain.ErrDeviceNotFound
		}
		return devicesdomain.Device{}, err
	}
	device.WorkspaceID = nullStringPtr(workspaceID)
	device.PushToken = nullStringPtr(pushToken)
	device.AppVersion = nullStringPtr(appVersion)
	device.BuildNumber = nullStringPtr(buildNumber)
	device.ReleaseChannel = nullStringPtr(releaseChannel)
	device.Locale = nullStringPtr(locale)
	device.Timezone = nullStringPtr(timezone)
	if revokedAt.Valid {
		device.RevokedAt = &revokedAt.Time
	}
	return device, nil
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
