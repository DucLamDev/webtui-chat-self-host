package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	usersapp "github.com/duclamdev/application-chat/backend/internal/modules/users/application"
	usersdomain "github.com/duclamdev/application-chat/backend/internal/modules/users/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) FindByID(ctx context.Context, id string) (usersdomain.User, error) {
	row := r.pool.QueryRow(ctx, `
SELECT id::text, email::text, username::text, display_name, avatar_url, phone_number, status,
       locale, timezone, email_verified_at, last_seen_at,
       host(registration_ip_address), registration_device_name,
       host(last_ip_address), device_name,
       created_at, updated_at
FROM users
WHERE id = $1::uuid AND deleted_at IS NULL
`, id)
	return scanUser(row)
}

func (r *Repository) FindByIDInZone(ctx context.Context, id string, zoneID string) (usersdomain.User, error) {
	row := r.pool.QueryRow(ctx, `
SELECT DISTINCT
       users.id::text, users.email::text, users.username::text, users.display_name,
       users.avatar_url, users.phone_number, users.status,
       users.locale, users.timezone, users.email_verified_at, users.last_seen_at,
       host(users.registration_ip_address), users.registration_device_name,
       host(users.last_ip_address), users.device_name,
       users.created_at, users.updated_at
FROM users
JOIN workspace_members member
  ON member.user_id = users.id
 AND member.status = 'active'
JOIN workspaces workspace
  ON workspace.id = member.workspace_id
 AND workspace.zone_id = $2::uuid
 AND workspace.status = 'active'
 AND workspace.deleted_at IS NULL
WHERE users.id = $1::uuid
  AND users.deleted_at IS NULL
LIMIT 1
`, id, zoneID)
	return scanUser(row)
}

func (r *Repository) List(ctx context.Context, params usersapp.ListUsersParams) ([]usersdomain.User, error) {
	rows, err := r.pool.Query(ctx, `
SELECT id::text, email::text, username::text, display_name, avatar_url, phone_number, status,
       locale, timezone, email_verified_at, last_seen_at,
       host(registration_ip_address), registration_device_name,
       host(last_ip_address), device_name,
       created_at, updated_at
FROM users
WHERE deleted_at IS NULL
  AND EXISTS (
      SELECT 1
      FROM workspace_members member
      JOIN workspaces workspace
        ON workspace.id = member.workspace_id
       AND workspace.zone_id = $4::uuid
       AND workspace.status = 'active'
       AND workspace.deleted_at IS NULL
      WHERE member.user_id = users.id
        AND member.status = 'active'
  )
  AND (
    $1 = ''
    OR display_name ILIKE '%' || $1 || '%'
    OR email::text ILIKE '%' || $1 || '%'
    OR username::text ILIKE '%' || $1 || '%'
    OR (
      regexp_replace($1, '\D', '', 'g') <> ''
      AND regexp_replace(COALESCE(phone_number, ''), '\D', '', 'g') LIKE '%' || regexp_replace($1, '\D', '', 'g') || '%'
    )
  )
  AND ($2 = '' OR status = $2)
ORDER BY created_at DESC
LIMIT $3
`, params.Query, params.Status, params.Limit, params.ZoneID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]usersdomain.User, 0)
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *Repository) UserBelongsToWorkspace(ctx context.Context, userID string, workspaceID string) (bool, error) {
	var belongs bool
	err := r.pool.QueryRow(ctx, `
SELECT EXISTS (
    SELECT 1
    FROM workspace_members
    WHERE user_id = $1::uuid
      AND workspace_id = $2::uuid
      AND status = 'active'
)
`, userID, workspaceID).Scan(&belongs)
	return belongs, err
}

func (r *Repository) UpdateProfile(ctx context.Context, params usersapp.UpdateProfileParams) (usersdomain.User, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE users
SET display_name = COALESCE($2, display_name),
    avatar_url = COALESCE($3, avatar_url),
    phone_number = COALESCE($4, phone_number),
    locale = COALESCE($5, locale),
    timezone = COALESCE($6, timezone)
WHERE id = $1::uuid AND deleted_at IS NULL
RETURNING id::text, email::text, username::text, display_name, avatar_url, phone_number, status,
          locale, timezone, email_verified_at, last_seen_at,
          host(registration_ip_address), registration_device_name,
          host(last_ip_address), device_name,
          created_at, updated_at
`, params.UserID, params.DisplayName, params.AvatarURL, params.PhoneNumber, params.Locale, params.Timezone)
	return scanUser(row)
}

func (r *Repository) UpdateUser(ctx context.Context, params usersapp.UpdateUserParams) (usersdomain.User, error) {
	row := r.pool.QueryRow(ctx, `
UPDATE users
SET display_name = COALESCE($2, display_name),
    avatar_url = COALESCE($3, avatar_url),
    phone_number = COALESCE($4, phone_number),
    locale = COALESCE($5, locale),
    timezone = COALESCE($6, timezone),
    status = COALESCE($7, status)
WHERE id = $1::uuid AND deleted_at IS NULL
RETURNING id::text, email::text, username::text, display_name, avatar_url, phone_number, status,
          locale, timezone, email_verified_at, last_seen_at,
          host(registration_ip_address), registration_device_name,
          host(last_ip_address), device_name,
          created_at, updated_at
`, params.UserID, params.DisplayName, params.AvatarURL, params.PhoneNumber, params.Locale, params.Timezone, params.Status)
	return scanUser(row)
}

func (r *Repository) DeleteUser(ctx context.Context, userID string) error {
	command, err := r.pool.Exec(ctx, `
UPDATE users
SET status = 'disabled', deleted_at = now()
WHERE id = $1::uuid AND deleted_at IS NULL
`, userID)
	if err != nil {
		return err
	}
	if command.RowsAffected() == 0 {
		return usersdomain.ErrUserNotFound
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (usersdomain.User, error) {
	var user usersdomain.User
	var avatarURL sql.NullString
	var phoneNumber sql.NullString
	var emailVerifiedAt sql.NullTime
	var lastSeenAt sql.NullTime
	var registrationIP sql.NullString
	var registrationDevice sql.NullString
	var lastIPAddress sql.NullString
	var deviceName sql.NullString

	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.Username,
		&user.DisplayName,
		&avatarURL,
		&phoneNumber,
		&user.Status,
		&user.Locale,
		&user.Timezone,
		&emailVerifiedAt,
		&lastSeenAt,
		&registrationIP,
		&registrationDevice,
		&lastIPAddress,
		&deviceName,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return usersdomain.User{}, usersdomain.ErrUserNotFound
		}
		return usersdomain.User{}, err
	}

	user.AvatarURL = nullStringPtr(avatarURL)
	user.PhoneNumber = nullStringPtr(phoneNumber)
	user.EmailVerifiedAt = nullTimePtr(emailVerifiedAt)
	user.LastSeenAt = nullTimePtr(lastSeenAt)
	user.RegistrationIP = nullStringPtr(registrationIP)
	user.RegistrationDev = nullStringPtr(registrationDevice)
	user.LastIPAddress = nullStringPtr(lastIPAddress)
	user.DeviceName = nullStringPtr(deviceName)
	return user, nil
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
